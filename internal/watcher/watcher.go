package watcher

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v3/process"
)

const pollInterval = 5 * time.Second

type processRecord struct {
	ProcessSnapshot
	LastSeen time.Time `json:"-"`
}

type domainRecord struct {
	DomainSnapshot
	LastSeen time.Time `json:"-"`
}

type connectionRecord struct {
	ConnectionSnapshot
	LastSeen time.Time `json:"-"`
}

type service struct {
	mu             sync.Mutex
	hardware       HardwareProfile
	polls          uint64
	pids           map[int32]*processRecord
	domains        map[string]*domainRecord
	connections    map[string]*connectionRecord
	networkSamples map[string]networkConnectionSample
	lookupCache    map[string]reverseLookupResult
	lookupPending  map[string]bool
	lookupQueue    chan<- string
}

func StartDetached() (int, error) {
	cmd := exec.Command(os.Args[0], "__run")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		return 0, err
	}

	return cmd.Process.Pid, nil
}

func RunDaemon() error {
	hardware, err := CollectHardwareProfile()
	if err != nil {
		return err
	}

	pidNamespace := currentPIDNamespace()

	if err := writeStatus(Status{PID: os.Getpid(), PIDNamespace: pidNamespace}); err != nil {
		return err
	}

	svc := &service{
		hardware:       hardware,
		pids:           make(map[int32]*processRecord),
		domains:        make(map[string]*domainRecord),
		connections:    make(map[string]*connectionRecord),
		networkSamples: make(map[string]networkConnectionSample),
		lookupCache:    make(map[string]reverseLookupResult),
		lookupPending:  make(map[string]bool),
	}
	svc.lookupQueue = startReverseLookupWorker(svc)

	if err := svc.pollOnce(); err != nil {
		return err
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := svc.pollOnce(); err != nil {
				_ = svc.persist(true)
			}
		case <-stop:
			signal.Stop(stop)
			if err := svc.persist(false); err != nil {
				return err
			}
			return removeStatus()
		}
	}
}

func (s *service) pollOnce() error {
	now := time.Now()

	processes, err := process.Processes()
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.pollProcessesLocked(processes, now)
	s.mu.Unlock()

	observations, err := collectNetworkSamples()
	if err == nil {
		s.mu.Lock()
		s.pollDomainsLocked(observations, now)
		s.mu.Unlock()
	}

	s.mu.Lock()
	s.polls++
	err = s.persist(true)
	s.mu.Unlock()
	return err
}

func (s *service) persist(running bool) error {
	summaryPID := 0
	if running {
		summaryPID = os.Getpid()
	}

	summary := Summary{
		PID:                    summaryPID,
		PIDNamespace:           s.pidNamespace(running),
		Running:                running,
		PollCount:              s.polls,
		TrackedProcessCount:    len(s.pids),
		Hardware:               s.hardware,
		TopProcess:             s.topProcess(),
		TopProcesses:           s.topProcesses(6),
		TrackedDomainCount:     len(s.domains),
		TrackedConnectionCount: len(s.connections),
		TopDomains:             s.topDomains(60),
		TopConnections:         s.topConnections(20),
	}

	return writeSummary(summary)
}

func (s *service) pidNamespace(running bool) string {
	if !running {
		return ""
	}
	return currentPIDNamespace()
}

func (s *service) pollProcessesLocked(processes []*process.Process, now time.Time) {
	current := make(map[int32]struct{}, len(processes))

	for _, proc := range processes {
		current[proc.Pid] = struct{}{}

		record := s.pids[proc.Pid]
		if record == nil {
			record = &processRecord{}
			record.PID = proc.Pid
			s.pids[proc.Pid] = record
		}

		record.Name = safeString(proc.Name)
		record.Command = safeString(proc.Cmdline)
		record.CPUPercent = safeFloat(proc.CPUPercent)
		record.MemoryRSSMB = readRSSMB(proc)
		record.PollsSeen++
		record.LastSeen = now
	}

	for pid, record := range s.pids {
		if _, ok := current[pid]; !ok && now.Sub(record.LastSeen) > pollInterval*2 {
			delete(s.pids, pid)
		}
	}
}

func (s *service) pollDomainsLocked(observations map[string]networkObservation, now time.Time) {
	for key, observation := range observations {
		sample := observation.Sample
		previous, ok := s.networkSamples[key]
		if ok {
			sample.RXBytes = diffOrCurrent(sample.RXBytes, previous.RXBytes)
			sample.TXBytes = diffOrCurrent(sample.TXBytes, previous.TXBytes)
		}
		s.networkSamples[key] = observation.Sample

		record := s.domains[observation.Domain]
		if record == nil {
			record = &domainRecord{}
			record.Domain = observation.Domain
			s.domains[observation.Domain] = record
		}
		record.DisplayName = s.cachedDisplayNameLocked(observation.Domain)
		record.RXBytes += sample.RXBytes
		record.TXBytes += sample.TXBytes
		record.LastSeen = now
		record.PollsSeen++

		connectionKey := observedConnectionKey(observation)
		connection := s.connections[connectionKey]
		if connection == nil {
			connection = &connectionRecord{}
			connection.PID = observation.PID
			connection.Domain = observation.Domain
			connection.Protocol = observation.Protocol
			s.connections[connectionKey] = connection
		}
		connection.ProcessName = s.processNameLocked(observation.PID)
		connection.DisplayName = record.DisplayName
		connection.RXBytes += sample.RXBytes
		connection.TXBytes += sample.TXBytes
		connection.LastSeen = now
		connection.PollsSeen++
	}

	for key := range s.networkSamples {
		if _, ok := observations[key]; !ok {
			delete(s.networkSamples, key)
		}
	}

	for domain, record := range s.domains {
		if now.Sub(record.LastSeen) > pollInterval*12 {
			delete(s.domains, domain)
		}
	}

	for key, record := range s.connections {
		if now.Sub(record.LastSeen) > pollInterval*12 {
			delete(s.connections, key)
		}
	}
}

func (s *service) topProcess() ProcessSnapshot {
	top := s.topProcesses(1)
	if len(top) == 0 {
		return ProcessSnapshot{}
	}
	return top[0]
}

func (s *service) topProcesses(limit int) []ProcessSnapshot {
	if len(s.pids) == 0 {
		return nil
	}

	records := make([]*processRecord, 0, len(s.pids))
	for _, record := range s.pids {
		records = append(records, record)
	}

	sort.Slice(records, func(i, j int) bool {
		if records[i].CPUPercent == records[j].CPUPercent {
			if records[i].MemoryRSSMB == records[j].MemoryRSSMB {
				return records[i].PollsSeen > records[j].PollsSeen
			}
			return records[i].MemoryRSSMB > records[j].MemoryRSSMB
		}
		return records[i].CPUPercent > records[j].CPUPercent
	})

	if limit > len(records) {
		limit = len(records)
	}

	top := make([]ProcessSnapshot, 0, limit)
	for _, record := range records[:limit] {
		top = append(top, record.ProcessSnapshot)
	}
	return top
}

func (s *service) topDomains(limit int) []DomainSnapshot {
	if len(s.domains) == 0 {
		return nil
	}

	records := make([]*domainRecord, 0, len(s.domains))
	for _, record := range s.domains {
		if !shouldIncludeDomainRecord(record) {
			continue
		}
		records = append(records, record)
	}

	if len(records) == 0 {
		return nil
	}

	sort.Slice(records, func(i, j int) bool {
		leftTotal := records[i].RXBytes + records[i].TXBytes
		rightTotal := records[j].RXBytes + records[j].TXBytes
		if leftTotal == rightTotal {
			if records[i].RXBytes == records[j].RXBytes {
				if records[i].TXBytes == records[j].TXBytes {
					return records[i].PollsSeen > records[j].PollsSeen
				}
				return records[i].TXBytes > records[j].TXBytes
			}
			return records[i].RXBytes > records[j].RXBytes
		}
		return leftTotal > rightTotal
	})

	if limit > len(records) {
		limit = len(records)
	}

	top := make([]DomainSnapshot, 0, limit)
	for _, record := range records[:limit] {
		top = append(top, record.DomainSnapshot)
	}
	return top
}

func shouldIncludeDomainRecord(record *domainRecord) bool {
	if record == nil {
		return false
	}

	if normalizeResolvedHost(record.DisplayName) != "" && !isPublicIP(record.DisplayName) {
		return true
	}

	return !isPublicIP(record.Domain) && normalizeResolvedHost(record.Domain) != ""
}

func (s *service) topConnections(limit int) []ConnectionSnapshot {
	if len(s.connections) == 0 {
		return nil
	}

	records := make([]*connectionRecord, 0, len(s.connections))
	for _, record := range s.connections {
		if !shouldIncludeConnectionRecord(record) {
			continue
		}
		records = append(records, record)
	}

	if len(records) == 0 {
		return nil
	}

	sort.Slice(records, func(i, j int) bool {
		leftTotal := records[i].RXBytes + records[i].TXBytes
		rightTotal := records[j].RXBytes + records[j].TXBytes
		if leftTotal == rightTotal {
			if records[i].PollsSeen == records[j].PollsSeen {
				if records[i].ProcessName == records[j].ProcessName {
					return records[i].Domain < records[j].Domain
				}
				return records[i].ProcessName < records[j].ProcessName
			}
			return records[i].PollsSeen > records[j].PollsSeen
		}
		return leftTotal > rightTotal
	})

	if limit > len(records) {
		limit = len(records)
	}

	top := make([]ConnectionSnapshot, 0, limit)
	for _, record := range records[:limit] {
		top = append(top, record.ConnectionSnapshot)
	}
	return top
}

func shouldIncludeConnectionRecord(record *connectionRecord) bool {
	if record == nil {
		return false
	}

	if normalizeResolvedHost(record.DisplayName) != "" && !isPublicIP(record.DisplayName) {
		return true
	}

	return !isPublicIP(record.Domain) && normalizeResolvedHost(record.Domain) != ""
}

func observedConnectionKey(observation networkObservation) string {
	return fmt.Sprintf("%d|%s|%s", observation.PID, observation.Protocol, observation.Domain)
}

func (s *service) processNameLocked(pid int32) string {
	if record := s.pids[pid]; record != nil {
		return record.Name
	}
	return ""
}

func (s *service) cachedDisplayNameLocked(host string) string {
	if !isPublicIP(host) {
		return ""
	}

	if result, ok := s.lookupCache[host]; ok && result.ready {
		return result.name
	}

	if s.lookupPending[host] {
		return ""
	}

	s.lookupPending[host] = true
	select {
	case s.lookupQueue <- host:
	default:
		s.lookupPending[host] = false
	}

	return ""
}

func diffOrCurrent(current, previous uint64) uint64 {
	if current >= previous {
		return current - previous
	}
	return current
}

func safeString(fn func() (string, error)) string {
	value, err := fn()
	if err != nil {
		return ""
	}
	return value
}

func safeFloat(fn func() (float64, error)) float64 {
	value, err := fn()
	if err != nil {
		return 0
	}
	return value
}

func readRSSMB(proc *process.Process) float64 {
	info, err := proc.MemoryInfo()
	if err != nil || info == nil {
		return 0
	}
	return float64(info.RSS) / (1024 * 1024)
}

func IsProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}

func CleanStaleStatus() error {
	status, err := LoadStatus()
	if err != nil {
		return err
	}
	if status.PID == 0 {
		return nil
	}
	if shouldVerifyProcessLiveness(status.PIDNamespace, currentPIDNamespace()) && IsProcessRunning(status.PID) {
		return nil
	}
	if !shouldVerifyProcessLiveness(status.PIDNamespace, currentPIDNamespace()) {
		return nil
	}
	return removeStatus()
}

func init() {
	if err := CleanStaleStatus(); err != nil && !errors.Is(err, os.ErrNotExist) {
		_ = err
	}
}

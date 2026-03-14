# dialtone-watcher

Original Prompt:

```
here's a new idea, tell me if this exists. An app that runs on your laptop or desktop linux or mac, and looks at what processing are running. It saves these data via api calls to dialtoneapp.com every few mins. And the user can see at web UI at dialtoneapp a profile for what they run all day. They can compare to other users with similiar hardware specs and lifestyles and see if they have any weird apps running or are doing something less efficient. Also could add wireshark like listening and also include a list of domains they are sending outgoing or incoming data to from and the amounts. Also compare with others for weird sites.
```

---

## 1. Tools that already monitor processes (locally)

These collect **exactly the data you’re talking about**, but they usually **don’t upload it to a shared comparison network**.

### Examples

* **Activity Monitor**
* **htop**
* **Glances**

They show:

* running processes
* CPU usage
* memory usage
* network usage

But they are **local tools**, not social/benchmark systems.

---

## 2. Tools that analyze outgoing network traffic

These already do the **Wireshark-like domain analysis** you mentioned.

### Examples

* **Wireshark**
* **Little Snitch**
* **GlassWire**

They show:

* which domains apps connect to
* how much bandwidth they use
* suspicious traffic

But again: **not shared across users.**

---

## 3. Tools that benchmark hardware across users

This is the **closest category to your comparison idea**.

### Examples

* **Geekbench**
* **UserBenchmark**

They let users compare:

* CPU performance
* GPU performance
* hardware specs

But they **don’t analyze daily workflows or processes**.

---

## 4. Tools that monitor behavior for security

These sometimes collect process + network activity across many machines.

Example:

* **CrowdStrike Falcon**

Enterprise EDR tools track:

* processes
* network connections
* suspicious binaries

But the data is **private to companies**, not a shared community dataset.

---

# The gap your idea hits

Your idea combines **four things that normally live in separate products:**

1. **Process monitoring**
2. **Network domain tracking**
3. **Cloud aggregation**
4. **Community comparison**

That last one is the interesting part.

Example insight a user could see:

```
MacBook Pro M3 users running VSCode:

Average daily apps:
- VSCode
- Slack
- Chrome
- Docker

You are also running:
- weird_helperd
- crypto-miner-x

⚠️ Only 0.2% of similar users run this process.
```

Or:

```
Your laptop contacted:

api.openai.com
github.com
cdn.segment.io
random-tracker.xyz   ⚠️ uncommon
```

---

# Why this doesn’t really exist yet

Two big reasons.

### 1. Privacy concerns

Uploading:

* running apps
* network domains
* system activity

…is **extremely sensitive telemetry**.

Users will ask:

> “Are you spying on my computer?”

Even Apple got pushback over telemetry.

---

### 2. Hard to normalize comparisons

You’d need to normalize for:

* hardware specs
* profession
* operating system
* dev vs non-dev
* background services

Otherwise comparisons become noisy.

---

# But there *is* a strong version of your idea

The best angle isn’t “monitor everything”.

The best angle is something like:

### **“The Strava for your computer.”**

Users opt-in and share:

* daily app usage
* CPU usage patterns
* dev tool stacks
* network endpoints

Then compare:

```
People like you (MacBook + developer):

Top apps today:
1. VSCode
2. Docker
3. Chrome
4. Slack
```

Or:

```
Your machine contacted 132 domains today
Average developer: 42
⚠️ You may have excessive telemetry apps.
```

---

# The *really interesting* twist

You hinted at this earlier.

If everyone runs a small agent, you get:

**a massive distributed sensor network.**

You could detect:

* malware spreading
* suspicious domains
* new trackers
* weird background apps

Basically:

**Crowdsourced endpoint intelligence.**

Think:

> CrowdStrike for normal people.

---

# Brutal honest take

The idea has potential **if the hook is right.**

But the current pitch:

> “Upload your processes and network traffic”

…won’t get installs.

You need a hook like:

* **“Find hidden spyware on your computer.”**
* **“See what apps are secretly sending data.”**
* **“Compare your computer health to others.”**
* **“Detect malware early.”**

The comparison feature becomes **secondary magic**.

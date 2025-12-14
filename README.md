<p align="center">
    <a href="https://pi-apps.io">
        <img src="https://github.com/pi-apps-go/pi-apps/blob/main/icons/proglogo.png?raw=true" alt="Pi-Apps logo">
    </a>
</p>
<p align="center">The most popular app store for Raspberry Pi computers. 100% free, open-source but rewritten in Go for faster performance and with in mind cross platform support.
<p align="center">
  <a href="https://github.com/pi-apps-go/pi-apps/blob/main/CHANGELOG.md">
    View changelog</a>
  |
  <a href="https://pi-apps-go.io/wiki/getting-started/apps-list/">
    Apps List</a>
  |
  <a href="https://github.com/pi-apps-go/pi-apps/issues/new?template=bug-report.yml">
    Report an error</a>
  |
  <a href="https://github.com/pi-apps-go/pi-apps/issues/new?template=suggestion.yml">
    Make a general suggestion</a>
  |
  <a href="https://github.com/pi-apps-go/pi-apps/issues/new?assignees=&labels=App+Request&template=app-suggestion.yml&title=EDIT+ME+Include+the+app+name">
    Suggest new app</a>
  |
  <a href="https://github.com/pi-apps-go/pi-apps/issues/new?assignees=&labels=App+Request%2CZip%2FPR+included&template=app-submission.yml&title=EDIT+ME+Include+the+app+name+and+if+it+is+a+Package+app+or+an+Install+based+App">
    Submit a new app</a>

<p align="center">
    <a href="https://github.com/pi-apps-go/pi-apps/stargazers"><img src="https://img.shields.io/github/stars/pi-apps-go/pi-apps" alt="stars"></a>
    <a href="https://github.com/pi-apps-go/pi-apps/network/members"><img src="https://img.shields.io/github/forks/pi-apps-go/pi-apps" alt="forks"></a>
    <a href="https://github.com/pi-apps-go/pi-apps/graphs/contributors"><img src="https://img.shields.io/github/contributors/pi-apps-go/pi-apps" alt="contributors"></a>
    <a href="https://github.com/pi-apps-go/pi-apps/pulls"><img src="https://img.shields.io/github/issues-pr/pi-apps-go/pi-apps" alt="prs"></a>
    <a href="https://github.com/pi-apps-go/pi-apps/issues?q=is%3Aopen+is%3Aissue+label%3Abug"><img src="https://img.shields.io/github/issues/pi-apps-go/pi-apps/bug?color=red&label=bugs"></a>
    <a href="https://github.com/pi-apps-go/pi-apps/issues?q=is%3Aopen+is%3Aissue+label%3A%22App+Request%22"><img src="https://img.shields.io/github/issues/pi-apps-go/pi-apps/App%20Request?color=Green&label=app%20requests"></a>
    <a href="https://github.com/pi-apps-go/pi-apps/blob/main/COPYING"><img src="https://img.shields.io/github/license/pi-apps-go/pi-apps" alt="license"></a>
    <a href="https://discord.gg/RXSTvaUvuu"><img src="https://img.shields.io/discord/770629697909424159.svg?color=7289da&label=Discord%20server&logo=discord" alt="Join the Discord server"></a>
    <img src="https://img.shields.io/github/go-mod/go-version/pi-apps-go/pi-apps" alt="go.mod Go version">
    <img src="https://img.shields.io/github/languages/count/pi-apps-go/pi-apps" alt="Language count">
    <a href="https://app.fossa.com/projects/git%2Bgithub.com%2Fpi-apps-go%2Fpi-apps?ref=badge_shield" alt="FOSSA Status"><img src="https://app.fossa.com/api/projects/git%2Bgithub.com%2Fpi-apps-go%2Fpi-apps.svg?type=shield"/></a>

<p align="center"><strong> Original made with &#10084; by <a href="https://github.com/Botspot">Botspot</a></strong>, <strong><a href="https://github.com/theofficialgman">theofficialgman</a></strong>, and <a href="https://github.com/pi-apps-go/pi-apps/graphs/contributors">contributors</a>, while rewrite with <strong><a href="https://github.com/matu6968">matu6968</a></strong>

<p align="center">
    Check out our website: (unavailable for the time being) <a href="https://pi-apps-go.io">pi-apps-go.io</a>
</p>

## Introduction
Installing software on Linux is easy... until it isn't.  
Many popular apps just don't appear in the `apt` repositories, and it's very easy for inexperienced users to mess up their OS trying to install such apps manually.  
**We're trying to solve this problem no matter on what architecture they are running.**  
Introducing Pi-Apps Go, a well-maintained collection of scripts that automatically install hard-to-install apps no matter if you are running a typical x86_64 laptop or a obscure RISC-V SBC. See the full list [here](https://pi-apps.io/wiki/getting-started/apps-list/).  

Pi-Apps Go is not your average app store. Rather than require any standardized packaging format (but it can in the future pack in a distro specific package for caching purposes) or centralized hosting, our cross platform shell scripts download the app from *where it already is*.  
Scripts offer tremendous flexibility. If you can manually install it, then Pi-Apps Go can automatically install it. [Please help us expand our list of apps.](https://pi-apps.io/wiki/development/Creating-an-app/) *You don't have to be the app developer to get it added to Pi-Apps!* (You just have to know a little bit of bash scripting)

Original Pi-Apps serves **over 1,000,000 people** and hosts [over 200 apps](https://pi-apps.io/wiki/getting-started/apps-list/).

**Pi-Apps Go is very new and is a work in progress.** Please expect some features to be missing/broken and [report](https://github.com/pi-apps-go/pi-apps/issues/new) any issues you encounter.

Currently it's unsuitable for use outside of developers due to being unstable, this will change soon.

Current stage: **Development**

Beta testing stage: **in around late 2025 - early 2026 or eariler** 

Stable stage: **around early 2026 or eariler**

## Differences between Pi-Apps and Pi-Apps Go
Pi-Apps Go is based on the same architecture as Pi-Apps, with many improvements added on top of it.
To give you an idea of the differences, here is a list of the differences:

<table border="2" cellspacing="0" cellpadding="6" rules="groups" frame="hsides">

<colgroup>
<col  class="org-left" />

<col  class="org-left" />

<col  class="org-left" />
</colgroup>
<thead>
<tr>
<th scope="col" class="org-left">Feature</th>
<th scope="col" class="org-left"><img src="https://github.com/Botspot/pi-apps/blob/master/icons/proglogo.png?raw=true" alt="Pi-Apps logo" height="60"></th>
<th scope="col" class="org-left"><img src="https://github.com/pi-apps-go/pi-apps/blob/main/icons/proglogo.png?raw=true" alt="Pi-Apps Go logo"  height="60"></th>
</tr>
</thead>

<tbody>
<tr>
<td class="org-left">Package manager support</td>
<td class="org-left">APT only</td>
<td class="org-left">APT, APK, pacman*, dnf* and many more</td>
</tr>


<tr>
<td class="org-left">Localization support</td>
<td class="org-left">❌ (English only)</td>
<td class="org-left">✅ (currently English, Spanish and Polish)</td>
</tr>


<tr>
<td class="org-left">Architecture support</td>
<td class="org-left">ARM only (unofficially x86)</td>
<td class="org-left">ARM, x86, RISC-V</td>
</tr>


<tr>
<td class="org-left">Speed</td>
<td class="org-left">❌ (bit slow, core Pi-Apps runtime written in Bash)</td>
<td class="org-left">✅ (written in Go, compiles to native code)</td>
</tr>

<tr>
<td class="org-left">Platform agnostic and portable</td>
<td class="org-left">❌ (depends on Linux only commands/utilties that won't work anywhere)</td>
<td class="org-left">✅ (Go libraries handle the platform abstraction)</td>
</tr>
</tbody>
</table>


<a id="support"></a>


## Install Pi-Apps Go
Open a terminal and run this command:
```bash
wget -qO- https://raw.githubusercontent.com/pi-apps-go/pi-apps/main/install | bash
```
<img src="icons/screenshots/main%20window.png?raw=true" align="right" height="270px"/>

### Supported systems:
#### Raspberry Pi <img src="https://pi-apps.io/img/other-icons/raspberrypi-icon.svg" height="14"> (2/3/Zero 2 W/4/5)
- <img src="https://pi-apps.io/img/other-icons/raspberrypi-icon.svg" height="14"> [**Raspberry Pi OS**](https://www.raspberrypi.com/software/operating-systems/) (32-bit/64-bit) (Bookworm/Trixie): <span style="color:var(--success-dark);">fully supported</span>
- <img src="https://pi-apps.io/img/other-icons/ubuntu-icon.svg" height="14"> [**Ubuntu**](https://ubuntu.com/download/raspberry-pi) (Jammy/Noble): <span style="color:var(--success-dark);">fully supported</span>
- <img src="https://archlinux.org/static/favicon.png" height="14"> [Arch Linux](https://archlinuxarm.org): <span style="color:var(--warn-dark);">Support is in progress</span>
- <img src="https://alpinelinux.org/alpine-logo.ico" height="14"> [Alpine Linux](https://alpinelinux.org): <span style="color:var(--warn-dark);">Support is in progress</span>
- <img src="https://chimera-linux.org/assets/icons/favicon48.png" height="14"> [Chimera Linux](https://chimera-linux.org): <span style="color:var(--warn-dark);">Support is in progress</span>
#### Nintendo Switch <img src=https://pi-apps.io/img/other-icons/switch-icon.svg height="14">
- <img src=https://pi-apps.io/img/other-icons/switchroot-icon.png height="14"> [**Switchroot L4T Ubuntu Noble (24.04)**](https://wiki.switchroot.org/wiki/linux/l4t-ubuntu-noble-installation-guide): <span style="color:var(--success-dark);">fully supported</span>
- <img src=https://pi-apps.io/img/other-icons/switchroot-icon.png height="14"> [**Switchroot L4T Ubuntu Jammy (22.04)**](https://wiki.switchroot.org/wiki/linux/l4t-ubuntu-jammy-installation-guide): <span style="color:var(--success-dark);">fully supported</span>
#### Nvidia Jetson <img src=https://pi-apps.io/img/other-icons/nvidia-icon.svg height="14">
- <img src=https://pi-apps.io/img/other-icons/nvidia-icon.svg height="14"> [**Nvidia Jetpack 6**](https://developer.nvidia.com/embedded/jetpack-sdk-62) (Ubuntu Jammy): <span style="color:var(--success-dark);">fully supported</span>
- <img src=https://pi-apps.io/img/other-icons/nvidia-icon.svg height="14"> [**Nvidia Jetpack 5**](https://developer.nvidia.com/embedded/jetpack-sdk-514) (Ubuntu Focal): <span style="color:var(--success-dark);">fully supported</span>
#### Apple Silicon Macs
- <img src="https://pi-apps.io/img/other-icons/ubuntu-icon.svg" height="14"> [**Ubuntu Asahi**](https://ubuntuasahi.org/) (Ubuntu Noble): <span style="color:var(--warn-dark);">Not actively tested but all available apps should work</span>
- <img src="https://archlinux.org/static/favicon.png" height="14"> [Arch Linux](https://archlinuxarm.org) (requires Arch Linux support plugin during installation): <span style="color:var(--warn-dark);">Support is in progress</span>
#### [**Pine64**](https://www.pine64.org/), [**Orange Pi**](http://www.orangepi.org/), [**Radxa**](https://rockpi.org/), [**Banana Pi**](https://banana-pi.org/), [**Khadas**](https://www.khadas.com/), [**Inovato**](https://www.inovato.com/), [**Libre Computer**](https://libre.computer/), and other **ARMv7/ARMv8/ARMv9** Devices
- <img src="https://pi-apps.io/img/other-icons/debian-icon.svg" height="14"> [Debian Bookworm/Trixie](https://www.debian.org/distrib/) (Official Releases from Debian **ONLY**): <span style="color:var(--warn-dark);">Not actively tested but all available apps should work</span>
- <img src="https://pi-apps.io/img/other-icons/ubuntu-icon.svg" height="14"> [Ubuntu Jammy/Noble](https://ubuntu.com/download/desktop) (Official Releases from Canonical **ONLY**): <span style="color:var(--warn-dark);">Not actively tested but all available apps should work</span>
- <img src="https://archlinux.org/static/favicon.png" height="14"> [Arch Linux](https://archlinuxarm.org) (Official Releases **ONLY**, requires Arch Linux support plugin during installation): <span style="color:var(--warn-dark);">Support is in progress</span>
- <img src="https://alpinelinux.org/alpine-logo.ico" height="14"> [Alpine Linux](https://alpinelinux.org): <span style="color:var(--warn-dark);">Support is in progress</span>
- <img src="https://chimera-linux.org/assets/icons/favicon48.png" height="14"> [Chimera Linux](https://chimera-linux.org): <span style="color:var(--warn-dark);">Support is in progress</span>

# Supported devices for the future:
#### [**Pine64**](https://www.pine64.org/), [**Orange Pi**](http://www.orangepi.org/), [**Radxa**](https://rockpi.org/), [**Banana Pi**](https://banana-pi.org/), [**Milk-V**](https://milkv.io/), [**SiFive**](https://www.sifive.com/), [**StarFive**](https://www.starfivetech.com/) and other **RISC-V** Devices
- <img src="https://pi-apps.io/img/other-icons/debian-icon.svg" height="14"> [Debian Trixie](https://www.debian.org/distrib/) (Official Releases from Debian **ONLY**): <span style="color:var(--warn-dark);">Support is planned for the future, currently unsupported</span>
- <img src="https://pi-apps.io/img/other-icons/ubuntu-icon.svg" height="14"> [Ubuntu Noble](https://ubuntu.com/download/risc-v) (Official Releases from Canonical **ONLY**): <span style="color:var(--warn-dark);">Support is planned for the future, currently unsupported</span>
- <img src="https://archlinux.org/static/favicon.png" height="14"> [Arch Linux](https://archriscv.felixc.at/): <span style="color:var(--warn-dark);">Support is planned for the future, currently unsupported</span>
#### Generic **x86_64** devices
- <img src="https://pi-apps.io/img/other-icons/debian-icon.svg" height="14"> [Debian Bookworm/Trixie](https://www.debian.org/distrib/) (Official Releases from Debian **ONLY**): <span style="color:var(--warn-dark);">Support is in progress</span>
- <img src="https://pi-apps.io/img/other-icons/ubuntu-icon.svg" height="14"> [Ubuntu Jammy/Noble](https://ubuntu.com/download/desktop) (Official Releases from Canonical **ONLY**): <span style="color:var(--warn-dark);">Support is in progress</span>
- <img src="https://archlinux.org/static/favicon.png" height="14"> [Arch Linux](https://archlinux.org): <span style="color:var(--warn-dark);">Support is in progress</span>
- <img src="https://alpinelinux.org/alpine-logo.ico" height="14"> [Alpine Linux](https://alpinelinux.org): <span style="color:var(--warn-dark);">Support is in progress</span>
- <img src="https://chimera-linux.org/assets/icons/favicon48.png" height="14"> [Chimera Linux](https://chimera-linux.org): <span style="color:var(--warn-dark);">Support is in progress</span>

# Supported operating systems for the future:
#### Nintendo Switch <img src=https://pi-apps.io/img/other-icons/switch-icon.svg height="14">
- <img src=https://pi-apps.io/img/other-icons/switchroot-icon.png height="14"> [Switchroot L4T Fedora 41](https://wiki.switchroot.org/wiki/linux/l4t-fedora-installation-guide-1): <span style="color:var(--success-dark);">Support is planned for the future, currently unsupported</span>

#### Everything else:

#### dnf based:
- <img src="https://fedoraproject.org/favicon.ico" height="14"> [Fedora](https://fedoraproject.org): <span style="color:var(--warn-dark);">Support is planned for the future, currently unsupported</span>
- <img src="https://redhat.com/favicon.ico" height="14"> [RHEL](https://redhat.com/rhel/): <span style="color:var(--warn-dark);">Support is planned for the future, currently unsupported</span>
- <img src="https://rockylinux.org/favicon.png" height="14"> [Rocky Linux](https://rockylinux.org): <span style="color:var(--warn-dark);">Support is planned for the future, currently unsupported</span>
- <img src="https://almalinux.org/fav/favicon.ico" height="14"> [AlmaLinux](https://almalinux.org): <span style="color:var(--warn-dark);">Support is planned for the future, currently unsupported</span>
- <img src="https://centos.org/assets/icons/favicon.svg" height="14"> [CentOS Stream](https://centos.org): <span style="color:var(--warn-dark);">Support is planned for the future, currently unsupported</span>

### Unsupported systems:
- Raspberry Pi Pico (2): <span style="color:var(--danger-dark);">These devices are microcontrollers and cannot run linux.
- All **UNOFFICIAL** Debian, Ubuntu and Arch based releases (unless mentioned above): <span style="color:var(--danger-dark);">Expect many apps to have issues.</span>
  - Examples: **Orange Pi OS**, <img src="https://pi-apps.io/img/other-icons/pop-os.svg" height="14"> [**Pop_OS!**](https://pop.system76.com/), **Kali Linux**, and **ChromeOS Crostini** Debian Container
- Anything Non-Debian, Non-Ubuntu, Non-Arch and Non-RHEL compatible: <span style="color:var(--danger-dark);">Expect the majority of apps and the appstore to be broken.</span>
  - Examples: **Slackware**, **SUSE**, **Gentoo**, **Void Linux**, **NixOS** 
- Anything not already mentioned: <span style="color:var(--danger-dark);">Expect nothing to work.</span>
  - Examples: **Microsoft Windows**, **MacOS**, **Android**, and **ChromeOS**

<details>
<summary><b>To install Pi-Apps manually</b> if you prefer to see what happens under the hood</summary>
 
```
git clone https://github.com/pi-apps-go/pi-apps
~/pi-apps/install
```
</details>

<details>
<summary><b>To uninstall Pi-Apps Go</b></summary>
This will not uninstall any apps that you installed through Pi-Apps Go.

```
~/pi-apps/uninstall
```
</details>

## To run Pi-Apps Go
- From the start menu: Accessories -> Pi-Apps
- Use the terminal command: `pi-apps-go`
- Run Pi-Apps from its directory: `~/pi-apps/gui`

## To update Pi-Apps Go
- Pi-Apps Go will automatically check for updates on boot and display a notification to update.
- To manually run the updater, use this command: `~/pi-apps/updater gui`
- It also supports a CLI interface: `~/pi-apps/updater cli`

## What do others say about Pi-Apps?
> Message from a **[pi-top](https://pi-top.com)** employee: "Happy to say that I recommend pi-apps to almost every school I work with when they start using Raspberry Pi"

> [Video by **ETA Prime**](https://www.youtube.com/watch?v=oqNWJ52DLes): "It's an awesome Raspberry Pi app store and it works really well and there's lots of great stuff in here and it's super easy to install. I want to give the devs of Pi-Apps a big shout-out."

> [Video by **leepspvideo**](https://www.youtube.com/watch?v=zxyWQ3FV98I): "Thanks so much to Botspot for creating this; it's a great program."

> [Video by **Novaspirit Tech**](https://youtu.be/9dO448vYv18?t=164) (RIP): "This is an awesome application for your Pi."

> Email from a **Raspberry Pi employee**: "I gave Pi-Apps a go a while back and have suggested it to others quite a few times.
> We can't provide all the options people may want, so it helps a lot that there are people like you who can help bridge the gap. Thank you Botspot!"

> [**ShiftPlusOne** (Raspberry Pi forum moderator)](https://www.raspberrypi.org/forums/viewtopic.php?f=63&t=290329&p=1755860#p1755857): "Thanks for the great work making it all simple for everybody."

> Email from [**Sakaki** (legendary RPi developer)](https://github.com/sakaki-): "Good luck with your projects, Botspot, you're really making waves!"

> Message from one of our many satisfied users: "Thank you for making pi-apps, it has helped me a ton (no more searching hours to find how to install etcher) and I cannot thank you enough."

## Basic usage
Pi-Apps Go is very easy to use.  
- This is the **main window**.  
![main window](icons/screenshots/main%20window.png?raw=true)  
  - ![icon](icons/screenshots/buttons/search.png?raw=true) Search for apps.
  - ![icon](icons/screenshots/buttons/settings.png?raw=true) Open pi-apps settings.
  - Click on a category to open it.
    
- Opening a category will reveal a **list of apps**:  
![app list](icons/screenshots/app%20list.png?raw=true)  
  - ![icon](icons/screenshots/buttons/back2.png?raw=true) Go back to the main list of categories.
  - Click on an App name to see its details (see **details window** below)

- On the right is the app **details window**:  
![details](icons/screenshots/details%20window.png?raw=true)  
  - ![icon](icons/screenshots/buttons/scripts.png?raw=true) View the shell-scripts responsible for installing or uninstalling the selected app.
  - ![icon](icons/screenshots/buttons/edit.png?raw=true) Modify the app's description, icons, or scripts. (This button is hidden unless you enable it in Settings)
  - ![icon](icons/screenshots/buttons/install.png?raw=true) Install the selected app.
  - ![icon](icons/screenshots/buttons/uninstall.png?raw=true) Uninstall the selected app.

- If you install/uninstall an app, you will see the **progress window**, keep installing/uninstalling apps to add them to the queue:  
![details](icons/screenshots/manage.png?raw=true)

- Pi-Apps Settings can be configured by launching Menu -> Preferences -> Pi-Apps Settings.  
![settings](icons/screenshots/settings.png?raw=true)  
In addition to changeable settings, this window also gives access to these tools:
  - ![icon](icons/screenshots/buttons/categories.png?raw=true) Does that one app seem to be in the wrong category? With this button, you can change it.
  - ![icon](icons/screenshots/buttons/new%20app.png?raw=true) Create a new app with a wizard-style set of dialogs. We recommend reading [the tutorial](https://pi-apps.io/wiki/development/Creating-an-app/).
  - ![icon](icons/screenshots/buttons/log%20files.png?raw=true) View the past weeks-worth of installation logs. This is useful if you ever encounter an app that won't install and want to see the terminal output after you closed the terminal.
  - ![icon](icons/screenshots/buttons/import%20app.png?raw=true) This allows you to easily import a 3rd-party app from elsewhere. It helps Pi-Apps developers test upcoming apps for reliability on a variety of systems.

To learn more about Pi-Apps Go, read [the documentation](https://pi-apps-go.io/wiki/development/DOCUMENTATION/) and the [wiki](https://pi-apps-go.io/wiki/).

## :raised_hands: Contributing
You don't need to be a programmer to help!  
- The easiest way to help is by "Starring" our repository - it helps more people find Pi-Apps. (Scroll to the top of this page and on the right there is a "Star" button)
- If you know somebody else who has a Raspberry Pi, feel free to tell them about Pi-Apps. We would offer you a referral discount, but Pi-Apps is free, so... `¯\_(ツ)_/¯`
- You can [make suggestions](https://github.com/pi-apps-go/pi-apps/issues/new?template=suggestion.yml), [report bugs](https://github.com/pi-apps-go/pi-apps/issues/new?template=bug-report.yml), or [suggest apps](https://github.com/pi-apps-go/pi-apps/issues/new?template=app-suggestion.yml).
- You can [create and submit an app](https://pi-apps.io/wiki/development/Creating-an-app/). Don't worry, it's about as easy as using the terminal (and it's even easier if you're submitting a simple apt-package)!
- You can also join our cheerful community: <a href="https://discord.gg/RXSTvaUvuu"><img src="https://img.shields.io/discord/770629697909424159.svg?color=7289da&label=Discord%20server&logo=discord" alt="Join the Discord server"></a>

## Badge
If your application is on Pi-Apps Go, please consider adding this pretty badge/hyperlink to your README (dark and light modes available):  

[![badge](https://github.com/pi-apps-go/pi-apps/blob/main/icons/badge.png?raw=true)](https://github.com/pi-apps-go/pi-apps)  [![badge](https://github.com/pi-apps-go/pi-apps/blob/main/icons/badge-light.png?raw=true)](https://github.com/pi-apps-go/pi-apps)

Embed code (Dark):
```
[![badge](https://github.com/pi-apps-go/pi-apps/blob/main/icons/badge.png?raw=true)](https://github.com/pi-apps-go/pi-apps)
```
Embed code (Light):
```
[![badge](https://github.com/pi-apps-go/pi-apps/blob/main/icons/badge-light.png?raw=true)](https://github.com/pi-apps-go/pi-apps)
```

### Q&A with matu6968
 - Why did you make Pi-Apps Go?
> I have been wanting to improve the speed of Pi-Apps considering I have been seeing slow downs on my Pi Zero 2W because it was written in Bash.

> There have been also projects such as [Pi-Apps Recreates](https://github.com/Pi-Apps-Recreates) which brang [x86 support](https://github.com/Pi-Apps-Recreates/pi-apps-x86) to Pi-Apps, but it stopped being updated in ~2023, so it's the time for a modern replacement.

> So one day I realized: Why not make a modern rewrite of Pi-Apps in Go because the language started getting traction in being used in backend development? This would speed up significantly since Go can compile to a native binary which can speed the thing up. 

 - How long did it take to program this?
> For now you would think it's still under development. 🤪

> It still is, Pi-Apps Go took around 3 days to re-implement the entire API script from bash to Go (with some features still being missing), and ever since then it has been continually, and exponentially, improving. I started to re-implement the GUI portions of Pi-Apps (such as the manage script) and for now it's still under development.

 - Is Pi-Apps Go free?
> Absolutely! [Donations are welcome](https://github.com/sponsors/matu6968), but Pi-Apps Go itself will always be free and open-source.

## API usage

Pi-Apps Go offers a Go module for other Go programs to use if Pi-Apps Go is installed.

### Example usage
To install it:
```bash
go get github.com/pi-apps-go/pi-apps/pkg/api
```

And then you can import the module with:

```go
package main

import "github.com/pi-apps-go/pi-apps/pkg/api"

func main() {
    api.Status("Hello, world!")
}
```

Another example (installing a package):
```go
api.InstallApp("Ruffle")
```

Another example (uninstalling a package):
```go
api.UninstallApp("Ruffle")
```

For the full API, see the ~~[API documentation](https://pkg.go.dev/github.com/pi-apps-go/pi-apps/pkg/api)~~ not yet available.

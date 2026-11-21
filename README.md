<div align="center">
<a href="https://github.com/nerdyslacker/Sysentra"><img src="assets/sysentra.png" width="100"/></a>
</div>
<h1 align="center">Sysentra</h1>
<div align="center">

[![License](https://img.shields.io/github/license/nerdyslacker/Sysentra)](https://github.com/nerdyslacker/Sysentra/blob/main/LICENSE)
[![GitHub release](https://img.shields.io/github/release/nerdyslacker/Sysentra)](https://github.com/nerdyslacker/Sysentra/releases)
<a href="https://app.commanddash.io/agent?github=https://github.com/nerdyslacker/Sysentra"><img src="https://img.shields.io/badge/AI-Code%20Assist-EB9FDA"></a>
[![GitHub stars](https://img.shields.io/github/stars/nerdyslacker/Sysentra)](https://github.com/nerdyslacker/Sysentra/stargazers)
[![GitHub forks](https://img.shields.io/github/forks/nerdyslacker/Sysentra)](https://github.com/nerdyslacker/Sysentra/fork)

</div>

Sysentra is a software and system management center for Void Linux and derivatives.

## Current features

- Search, install, remove, update, hold, and repository-lock XBPS packages.
- Browse and manage Flatpak applications from Flathub.
- Install, safely remove, purge, and reconfigure Linux kernels; inspect boot images, boot/EFI capacity, bootloader, and DKMS state.
- Manage system and user runit services.
- Manage XBPS repositories, mirrors, and package rules.
- Inspect firmware and hardware drivers.
- Inspect firewall rules and manage Samba shares, NFS mounts, users, and groups.
- Preview privileged changes before polkit authorization.
- Choose light, dark, or system appearance with selectable themes and fonts.

## Screenshots


<div align="center">
    <img src="assets/screenshots/main.png" alt="Installed package details" />
</div>

<details>
<summary><b>More</b></summary>
<br />

| Discover | Repositories | Mirrors |
|---|---|---|
|  <img src="assets/screenshots/discover.png" alt="Discover" /> | <img src="assets/screenshots/repositories.png" alt="Repositories" /> | <img src="assets/screenshots/mirrors.png" alt="Mirrors" /> |

| Services | Users & Groups | Kernels |
|---|---|---|
| <img src="assets/screenshots/services.png" alt="Services" /> | <img src="assets/screenshots/users_groups.png" alt="UsersAndGroups" /> | <img src="assets/screenshots/kernels.png" alt="Kernels" /> |

| Firmware | Drivers | Settings |
|---|---|---|
| <img src="assets/screenshots/firmware.png" alt="Firmware" /> | <img src="assets/screenshots/drivers.png" alt="Drivers" /> | <img src="assets/screenshots/settings.png" alt="Settings" /> |

</details>


## Installation

First add LazyLinux repository:

```sh
printf '%s\n' 'repository=https://github.com/lazylinuxos/lazy-repo/releases/latest/download' \
  | sudo tee /etc/xbps.d/99-repository-lazy.conf
```

Now execute install command:

```sh
sudo xbps-install -S sysentra
```

## Build dependencies

- `base-devel`
- `go` 1.26 or newer
- `nodejs`
- `gtk4-devel`
- `libwebkitgtk60-devel`
- Wails v3 CLI

## Runtime dependencies

- `gtk4`
- `libwebkitgtk60`
- `xbps`
- `runit`
- `dbus`
- `polkit`

Optional features use `flatpak`, `fwupd`, `turnstile`, `ufw` or `firewalld`, `samba`, and `nfs-utils`.

## Build

```sh
cd frontend
npm ci
npm run check
cd ..
wails3 generate bindings -ts .
cd frontend
npm run build
cd ..
go build -buildvcs=false -o sysentra .
go build -buildvcs=false -o bin/sysentrad ./cmd/sysentrad
```

Install `bin/sysentrad`, the D-Bus service file, D-Bus policy, and polkit policy from `packaging/` to enable privileged actions.

A starter Void Linux package template is available at `packaging/void/template`. Set its release version and checksum after publishing a tagged source archive.

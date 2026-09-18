---
name: Bug Report
about: Report a problem encountered while using ascend-device-plugin
labels: bug
---

<!-- Please use this template while reporting a bug and provide as much info as possible. Not doing so may result in your bug not being addressed in a timely manner. Thanks!
-->

**What happened**:

**What you expected to happen**:

**How to reproduce it (as minimally and precisely as possible)**:

**Anything else we need to know?**:

- Relevant excerpts from `npu-smi info`; mask NPU serial numbers, device identifiers, and host or node identifiers
- Relevant Docker or containerd configuration sections. Omit credentials, tokens, passwords, private keys, certificates, and unrelated host data.
- Relevant, time-bounded excerpts from the ascend-device-plugin container logs
- Relevant, time-bounded excerpts from the HAMi scheduler or Volcano scheduler container logs
- Relevant, time-bounded excerpts from the kubelet logs on the node (e.g: `sudo journalctl -r -u kubelet`)
- The relevant Helm values, ConfigMaps, or deployment manifests
- Relevant, time-bounded kernel output lines from `dmesg`

Before posting, remove or mask credentials, tokens, NPU identifiers, node or host identifiers, and other sensitive data from configuration and logs.

**Environment**:
- ascend-device-plugin version:
- Ascend NPU model:
- Ascend driver and CANN version:
- Kubernetes version:
- HAMi or Volcano version:
- Docker or containerd version:
- Installation method, image, and tag used:
- Slicing mode (`vNPU` or `hami-vnpu-core`):
- Kernel version from `uname -a`:
- Others:

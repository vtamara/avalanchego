# Proxy VM

Implements a simple managed VM intended to serve as a proxy to an
unmanaged VM. Whereas the lifecycle of a managed VM is tied to the
node - and debugging with delve is complicated - an unmanaged VM can
be updated and restarted without node restart. This also simplifies
debugging.

# WebSocket Internal Modules

Dependencies flow inward through capability interfaces:

```text
cmd/ws -> app -> httpserver/session -> room
               -> transport          -> WebSocket I/O
              -> cluster            -> relay
              -> persistence        -> store
              -> apiclient          -> HTTP API
```

`room` owns the authoritative state machine. It accepts authenticated identity,
session IDs, and dependency capabilities but does not import cluster, relay,
WebSocket, HTTP, or Redis implementations.
Local and edge-relayed commands enter the same room command path.

Locking rules for `room`:

- Public entry points acquire and release their own locks.
- Helpers ending in `Locked` require the room lock and leave it held.
- Network delivery and external adapter calls happen after releasing the room
  lock, normally through a notification closure returned by a locked helper.
- A state mutation persists its snapshot before the notification closure delivers
  the resulting outbound messages.

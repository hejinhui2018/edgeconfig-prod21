# EdgeConfig Project Brief

EdgeConfig is a local-first Go service for publishing versioned configuration to edge devices. The system persists domain events and snapshots, restores rollout state after restart, exposes HTTP and CLI entry points, and lets agents report the progress of an assignment through health confirmation.

The important business boundary is the relationship between device progress events and rollout aggregate status. A recovered process must present the same operational state as a process that remained online, including the final completed status of a rollout whose device has passed health checks.

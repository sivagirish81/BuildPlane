# Autoscaling

CPU-based HPA is insufficient for queued workloads because idle workers can show low CPU while the queue is growing. BuildPlane scales from backlog signals: queue depth, oldest queued age, active jobs, pending Kubernetes jobs, estimated duration, and requested CPU.

The decision engine computes required compute seconds and divides by target queue drain time. It then applies min/max capacity, scale-up and scale-down steps, cooldowns, pending pod accounting, and an age override so old queued work takes precedence over shallow depth.

The MVP exports desired capacity as a metric. Production should connect this target to Karpenter, Cluster Autoscaler, managed node pools, or an execution-token controller.

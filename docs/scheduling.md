# Scheduling

FIFO alone is insufficient because one tenant can flood the queue, large jobs can block smaller urgent work, and low-priority work needs bounded progress without competing equally with critical work.

BuildPlane uses priority weights:

- critical: 8
- high: 4
- normal: 2
- low: 1

Within a priority class, tenant fairness uses deficit round-robin. Each active tenant gains deficit according to its tenant scheduling weight. Job cost is requested CPU with a minimum cost of one. A tenant can schedule when its deficit covers the cost, then the cost is subtracted.

Priority is service class. Tenant weight is fair-share allocation within the active class. A high-weight tenant receives more long-term share than a low-weight tenant, but tenant and global concurrency limits prevent monopolization.

Starvation prevention comes from weighted priority cycling and accumulating tenant deficit. Large jobs wait until enough deficit accrues; small jobs can pass them without permanently excluding them.

Each scheduling iteration is `O(P*T*J log J)` for the bounded candidate snapshot, where `P` is priority classes, `T` active tenants, and `J` candidates per tenant. The production next step is partitioned queues and batched claims.

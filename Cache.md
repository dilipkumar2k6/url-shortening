# How much cache memory should we have?
- We can start with 20% of daily traffic and, based on clients’ usage pattern  we can adjust how many cache servers we need.
# Which cache eviction policy would best fit our needs?
Least Recently Used (LRU) can be a reasonable policy for our system.
To further increase the efficiency, we can replicate our caching servers to distribute the load between them.
# How can each cache replica be updated?

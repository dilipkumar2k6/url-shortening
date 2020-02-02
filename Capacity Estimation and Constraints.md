# Assumption
- Active users per month = 500 million
- Read vs Write ratio = 100:1
- Object size = 1 kb
- Request made by each users in month to create new url = 2 to 5 ~= 5
- If we follow the 80-20 rule, meaning 20% of URLs generate 80% of traffic, we would like to cache these 20% hot URLs.

# Traffic estimate
Estimated request per month = 500 * 5 million = 2.5 billion

Write request per seconds = 2500 million / (30 days * 24 hours * 60 mins * 60 seconds) ~= 1000 URLs/second ~= 1k per second

Total read per month= 2.5 * 100 billion = 250 billion

Read per second = 1k * 100 ~= 100k urls per second

# Storage estimate
Total number of objects in 5 years = 2.5 billion * 12 months * 5 years = 150 billion

Total storage = 150 billion * 1 kb = 150 million mb = 150 k gb = 150tb

# Network bandwidth 
Write (incoming) request per second = 1000 per seconds
Size per request = 1kb
Write (incoming) network bandwidth = 1000kb per second = 1mb per second

Read (out going) request per second = 100mb per second

Total network bandwidth = 100mb + 1mb ~= 100mb

# Memory estimates
Read request per second = 100k urls per second
Read request per day = 100k * 24 * 60 * 60 = 8640000 k = 8640 million = 8.6 billion 
To cache 20% of request = 8.6 billion * 0.20 = 1.72 billion
Size of cache request = 1.720 billion * 1 kb = 1720GB







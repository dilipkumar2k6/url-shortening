# Requirements
## Functional Requirements
- Given a URL, our service should generate a shorter and unique alias of it.
- When users access a short link, our service should redirect them to the original link.
- Users should optionally be able to pick a custom short link for their URL.
- Links will expire after a standard default time span. Users should be able to specify the expiration time.
## Non-Functional Requirements:
- The system should be highly available. This is required because, if our service is down, all the URL redirection will start failing.
- URL redirection should happen in real-time with minimal latency.
- Shortened links should not be guessable (not predictable).
## Extended Requirements:
- Analytics; e.g., how many times a redirection happened?
- Our service should also be accessible through REST APIs by other services.
# Volume constraints
- We will have 500M new URL shortenings per month
- 100:1 read/write ratio
# High level system design
![](https://docs.google.com/drawings/d/e/2PACX-1vQjXxAhOgIp7tfZKi2ynaMifyEbGr-326TanEQpIfrGz2f1CconN70wamUWmdJvfpRJJpZxpvKb863Z/pub?w=960&h=720)

# API Design
## Create user
```
POST /users?apiKey=string
{
    name: string
    email: string
    dob: datetime
}
```
It will write data in following schema
```
{
    userId: int
    name: string
    email: string
    dob: datetime
    createdAt: datetime
    lastLoggedIn: datetime
}
```
## Create Shorten Url
```
POST /shortenurl?apiKey=string&authToken=string
{
    originalUrl: string
    customAlias: string?
    expireAt: datetime?
}
Response: 
{
    shortenedUrl: string
}
```
This should write data in following schema
```
{
    shortenedUrl: string
    originalUrl: string
    customAlias: string?
    expiredAt: datetime?
    createdAt: datetime
    userId: int
}
```
## Delete Shortened Url
```
DELETE /shortenurl/<shortenedUrl>?apiKey=string&authToken=string
Response: 200
```
## Read URL and do redirection
```
GET /shortenurl/<shortenedUrl>?apiKey=string&authToken=string
Response: 302
```
## How to prevent API misuse?
- A malicious user can put us out of business by consuming all URL keys in the current design.
- To prevent abuse, we can limit users via their api_dev_key.
- Each api_dev_key can be limited to a certain number of URL creations and redirection per some time period (which may be set to a different duration per developer key).

# Algorithm to generate shortened Url
## Encode actual url
### Approach
- hash using sha256 which will generate 256 bit i.e. 64 hex string
- Then shorten it using base62 to use chars in range of a-z, A-Z, 0-9
- Why not base64? 
    - Let's not use `+` and `/` as these have special meaning in url
Q. What should be length of shortened url?
- 6 letter long key = 62^6 = ~ 56.8 billion
- 8 letter long key = 62^8 = ~ 218 trillion
Let's go with 8 letter long
- After `base62` on 64 digits of sha256 hash, it will generate `85` chars long key
- Get first 8 chars from encoded `base62` string
    - This will result into duplication
    - How to resolve it?
        - Reduce chars from encode list
        - swap some char
        - we have to keep generating a key until we get a unique one.
### What are the different issues with our solution?
- If multiple users enter the same URL, they can get the same shortened URL, which is not acceptable.
    - We can append an increasing sequence number to each input URL to make it unique and then generate its hash.
        - We don’t need to store this sequence number in the databases, though.
        - Possible problems with this approach could be an ever-increasing sequence number.
        - Can it overflow?
        - Appending an increasing sequence number will also impact the performance of the service.
    - Another solution could be to append the user id (which should be unique) to the input URL.
    - However, if the user has not signed in, we would generate a random key
    - Even after this, if we have a conflict, we have to keep generating a key until we get a unique one.
- What if parts of the URL are URL-encoded? e.g., `http://www.educative.io/distributed.php?id=design`, and `http://www.educative.io/distributed.php%3Fid%3Ddesign` are identical except for the URL encoding.
    - Apply default url decode on url before we use it to generate key
### we have to keep generating a key until we get a unique one

## Generating keys offline
### Approach
- We can have a standalone Key Generation Service (KGS) that generates random eight-letter strings beforehand and stores them in a database (let’s call it key-DB)
- Whenever we want to shorten a URL, we will take one of the already-generated keys and use it.
- This approach will make things quite simple and fast.
- Not only are we not encoding the URL, but we won’t have to worry about duplications or collisions.
- KGS will make sure all the keys inserted into key-DB are unique
### Can concurrency cause problems?
- As soon as a key is used, it should be marked in the database to ensure that it is not used again.
- If there are multiple servers/thread reading keys concurrently, we might get a scenario where two or more servers/thread try to read the same key from the database.
### How can we solve this concurrency problem?
- Servers can use KGS to read/mark keys in the database. 
    - KGS can use two tables to store keys: one for keys that are not used yet, and one for all the used keys. 
    - As soon as KGS gives keys to one of the servers, it can move them to the used keys table. 
- KGS can always keep some keys in memory to quickly provide them whenever a server needs them.
    - For simplicity, as soon as KGS loads some keys in memory, it can move them to the used keys table. 
    - This ensures each server gets unique keys. If KGS dies before assigning all the loaded keys to some server, we will be wasting those keys–which could be acceptable, given the huge number of keys we have.
- KGS also has to make sure not to give the same key to multiple servers. 
    - For that, it must synchronize (or get a lock on) the data structure holding the keys before removing keys from it and giving them to a server.
### What would be the key-DB size?
- 8 letter long key with base62 = 62^8 = ~ 218 trillion
- 2 byte to store one char
- Size of each key = 8 * 2 bytes = 16 bytes
- Total size = 218 trillion * 16 bytes ~ 3488 TB 
### Isn’t KGS a single point of failure?
- Yes, it is.
- To solve this, we can have a standby replica of KGS.
- Whenever the primary server dies, the standby server can take over to generate and provide keys.
### Can each app server cache some keys from key-DB?
- Yes, this can surely speed things up. 
- Although, in this case, if the application server dies before consuming all the keys, we will end up losing those keys. 
- This can be acceptable since we have 218 Trillion unique eight-letter keys.
## How would we perform a key lookup? 
- We can look up the key in our database to get the full URL. 
- If it’s present in the DB, issue an “HTTP 302 Redirect” status back to the browser, passing the stored URL in the “Location” field of the request. 
- If that key is not present in our system, issue an “HTTP 404 Not Found” status or redirect the user back to the homepage.
# Should we impose size limits on custom aliases? 
- Our service supports custom aliases. 
- Users can pick any ‘key’ they like, but providing a custom alias is not mandatory.
- However, it is reasonable (and often desirable) to impose a size limit on a custom alias to ensure we have a consistent URL database. 
- Let’s assume users can specify a maximum of 8 characters per customer key (as reflected in the above database schema).

# Storage Capacity Estimation and Constraints
## Shortened URL Estimate
### Database Schema
```
{
    shortenedUrl: string 8 chars ~ 16 bytes
    originalUrl: string 256 chars ~ 512 bytes
    customAlias: string? 8 chars ~ 16 bytes
    expiredAt: datetime? 8 bytes
    createdAt: datetime 8 bytes
    userId: int 4 bytes
}
```
### Storage estimation
- Size for each document: 16 + 512 + 16 + 8 + 8 + 4 ~ 564 bytes
- Size for key: 6*20 bytes = 120 bytes
- Size for index on  shortenedUrl = 16 bytes
- Total size for one document: 700 bytes 
- We will have 500M new URL shortenings per month
- 100:1 read/write ratio
- Total number of objects in 5 years: 500 million * 5 years * 12 months = 30 billion
- Total storage: 30 billion * 700 bytes ~ 21Tb
### Shard Storage
- Storage limit per server: 2TB
- Number of shards: 11
- Shard Key: shortenedUrl
- Number of replicas per shard for failover: 3
- Total storage: 63Tb
- CAP - AP system
## User Estimate
### Database schema
```
{
    userId: int 4 bytes
    name: string 120 bytes
    email: string 120 bytes
    password: string 120 bytes
    dob: datetime 8 bytes
    createdAt: datetime 8 bytes
    lastLoggedIn: datetime 8 bytes
}
```
### Storage estimation
- Size for each document: 4 + 120 + 120 + 120 + 8 + 8 + 8 ~ 388 bytes
- Size for key: 7*20 bytes = 140 bytes
- Size for index on  userId = 4 bytes
- Total size for one document: 532 bytes ~600 bytes
- We will have 500M new URL shortenings per month
- 10% user does registration
- Number of users: 50M
- Total number of objects in 5 years: 50 million * 5 years * 12 months = 300 million
- Total storage: 300 million * 600 bytes ~ 180 gb
- CAP - AP system
### Shard Storage
- Storage limit per server: 2TB
- Number of shards: 1
- Shard Key: userId
- Number of replicas per shard for failover: 3
- Total storage: 3Tb

# Traffic estimate
- 500M new URL shortenings per month
- 100:1 read/write ratio
- Expected redirection in month: 500M * 100 ~ 50 billion
- Query Per seconds for write: 500 million / (30 days * 24 hours * 3600 seconds) ~ ~200 URLs/s
    - Avg server can handle 200 thread per second
    - For failover, add three servers
- Considering 100:1 read/write ratio, URLs redirection per second will be: 100 * 200 URLs/s = 20K/s
    - Avg server can handle 200 thread per second
    - Number of app server: 20000/200 = 100 servers
# Network bandwidth estimate
## Write request
- Write (incoming) request per second = 200 per seconds
- Size per request = 500bytes
- Write (incoming) network bandwidth = 200 * 500 bytes = 100 KB/s

## Read request
- Read (out going) request per second = 20k per second
- Size per request = 500bytes
- Total network bandwidth = 20K * 500 bytes = ~10 MB/s

## Memory estimates
If we want to cache some of the hot URLs that are frequently accessed, how much memory will we need to store them?
- If we follow the 80-20 rule, meaning 20% of URLs generate 80% of traffic, we would like to cache these 20% hot URLs.
- We have 20K requests per second. Requests per day:
    - 20K * 3600 seconds * 24 hours = ~1.7 billion
- To cache 20% of these requests: 
    - 0.2 * 1.7 billion * 500 bytes = ~170GB

# How to handle if user uses shortened url to short it?
- Detect the pattern based on domain or shortened url
- Or allow 10 level of depth
# Purging or DB cleanup
If we chose to actively search for expired links to remove them, it would put a lot of pressure on our database. Instead, we can slowly remove expired links and do a lazy cleanup. 
- Whenever user tries to access a expired link, we can delete link and return error to the user
- A separate cleanup service can run periodically to remove expired links from storage and cache. 
- We can have default expiration on each link (for eg 2 years)
- After removing expired link, we can put that key back into key-DB to be reused.
# Security and Permissions
- Can users create private URLs or allow a particular set of users to access a URL?
- We can store the permission level (public/private) with each URL in the database. 
- We can also create a separate table to store UserIDs that have permission to see a specific URL.
- If a user does not have permission and tries to access a URL, we can send an error (HTTP 401) back. 
- Given that we are storing our data in a NoSQL wide-column database like Cassandra, the key for the table storing permissions would be the ‘Hash’ (or the KGS generated ‘key’). 
- The columns will store the UserIDs of those users that have the permission to see the URL.

# Analytics
- How many times short URL was used?
- What was the location of user?
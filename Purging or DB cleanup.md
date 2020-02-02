# Purging or DB cleanup
If we chose to actively search for expired links to remove them, it would put a lot of pressure on our database. Instead, we can slowly remove expired links and do a lazy cleanup. 
- Whenever user tries to access a expired link, we can delete link and return error to the user
- A separate cleanup service can run periodically to remove expired links from storage and cache. 
- We can have default expiration on each link (for eg 2 years)
- After removing expired link, we can put that key back into key-DB to be reused.

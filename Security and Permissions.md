# Security and Permissions
- Can users create private URLs or allow a particular set of users to access a URL?
- We can store the permission level (public/private) with each URL in the database. 
- We can also create a separate table to store UserIDs that have permission to see a specific URL.
- If a user does not have permission and tries to access a URL, we can send an error (HTTP 401) back. 
- Given that we are storing our data in a NoSQL wide-column database like Cassandra, the key for the table storing permissions would be the ‘Hash’ (or the KGS generated ‘key’). 
- The columns will store the UserIDs of those users that have the permission to see the URL.


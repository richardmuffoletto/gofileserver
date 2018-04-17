## File Storage Server Demonstration

Running the Demonstration
=========================

This demonstration uses 2 Bolt databases for storing user authentication and file data.

They will both be created during initialization, if they don't already exist, in the current directory.



Interacting with the server
===========================


Register a User
---------------

```
curl -v -s -d '{"username":"Bob", "password":"B1234567"}'  http://localhost:8080/register
```


Login a User
------------

```
curl -v -s -d '{"username":"Bob", "password":"B1234567"}'  http://localhost:8080/login
```



List Files
----------

```
curl -s -v -H "X-Session: b6161199-6ff7-4be4-90d6-12ff347a1fca" http://localhost:8080/files
```


Upload File
-----------

```
curl -s -v -X PUT -H "Content-Type: text/html" -H "X-Session: b6161199-6ff7-4be4-90d6-12ff347a1fca" -d @testfiles/a.txt http://localhost:8080/files/a.txt
```


Get File
--------

```
curl -s -v -H "X-Session: b6161199-6ff7-4be4-90d6-12ff347a1fca" http://localhost:8080/files/a.txt
```


Delete File
-----------

```
curl -s -v -X DELETE -H "X-Session: b6161199-6ff7-4be4-90d6-12ff347a1fca" http://localhost:8080/files/a.txt
```



Limitations and Next Steps
==========================

- Currently files are stored completely in memory. Change this to instead stream the data.
- Introduce interfaces for the authentication and files packages to allow them to be swapped out.
- Unit tests.
- Add a TTL to authentication tokens.
- Add logging.

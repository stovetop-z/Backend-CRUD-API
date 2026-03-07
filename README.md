# Backend API Example
---
This is some of the learning I have done with Go to create a backend CRUD API.

## API Support
This API has the following routes:
- signup
- login
- logout
- check-auth
- upload
- delete
- photos
- search
- media <- for file server

## How to Use
There is a make file that will install the necessary c files for date extraction from the uploaded photos. Additionally you need the following:
- Golang version >= 2.4
- g++
- Gemini API key
- MySQL

## The .env
The following MUST be in the .env to use the program:
DB_USER
DB_PASSWORD
EMAILS <- ONLY the specified emails are allowed to create accounts
PORT
IMG_PATH
MEDIA_PATH <- points to root dir
API_KEY

## Conclusion
This was a fun project to showcase some backend work I have been learning that communicates with a MySQL server and a frontend. 
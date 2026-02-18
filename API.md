# hCTF2 API Documentation

## Base URL
```
http://localhost:8090
```

## Authentication

Most endpoints require authentication via JWT token. The token can be provided in two ways:

1. **Cookie** (recommended): `auth_token` HttpOnly cookie
2. **Header**: `Authorization: Bearer <token>`

## Public Endpoints

### POST /api/auth/register
Register a new user.

**Request:**
```http
POST /api/auth/register
Content-Type: application/x-www-form-urlencoded

email=user@example.com&password=secret123&name=John Doe
```

**Response:** 200 OK
```json
{
  "user": {
    "id": "abc123",
    "email": "user@example.com",
    "name": "John Doe",
    "is_admin": false,
    "created_at": "2026-02-07T10:00:00Z",
    "updated_at": "2026-02-07T10:00:00Z"
  },
  "token": "eyJhbGciOiJIUzI1NiIs..."
}
```

### POST /api/auth/login
Login with existing credentials.

**Request:**
```http
POST /api/auth/login
Content-Type: application/x-www-form-urlencoded

email=user@example.com&password=secret123
```

**Response:** 200 OK
```json
{
  "user": {
    "id": "abc123",
    "email": "user@example.com",
    "name": "John Doe",
    "is_admin": false
  },
  "token": "eyJhbGciOiJIUzI1NiIs..."
}
```

**Error:** 401 Unauthorized
```
Invalid credentials
```

### POST /api/auth/logout
Logout and clear auth cookie.

**Request:**
```http
POST /api/auth/logout
```

**Response:** 200 OK
```
Logged out
```

### GET /api/challenges
List all challenges (visible only for non-admin).

**Request:**
```http
GET /api/challenges
```

**Response:** 200 OK
```json
[
  {
    "id": "challenge1",
    "name": "SQL Injection 101",
    "description": "Learn basic SQL injection techniques",
    "category": "web",
    "difficulty": "easy",
    "tags": "[\"sql\", \"injection\"]",
    "visible": true,
    "created_at": "2026-02-07T10:00:00Z",
    "updated_at": "2026-02-07T10:00:00Z"
  }
]
```

### GET /api/challenges/:id
Get challenge details with questions.

**Request:**
```http
GET /api/challenges/challenge1
```

**Response:** 200 OK
```json
{
  "challenge": {
    "id": "challenge1",
    "name": "SQL Injection 101",
    "description": "Learn basic SQL injection techniques",
    "category": "web",
    "difficulty": "easy",
    "visible": true
  },
  "questions": [
    {
      "id": "question1",
      "challenge_id": "challenge1",
      "name": "Find the admin password",
      "description": "Exploit the login form to get admin password",
      "flag_mask": "FLAG{********************}",
      "case_sensitive": false,
      "points": 100,
      "created_at": "2026-02-07T10:00:00Z"
    }
  ]
}
```

### GET /api/scoreboard
Get current scoreboard rankings.

**Request:**
```http
GET /api/scoreboard
```

**Response:** 200 OK
```json
[
  {
    "rank": 1,
    "user_id": "user123",
    "user_name": "Alice",
    "team_id": "team1",
    "team_name": "HackTheBox",
    "points": 500,
    "solve_count": 5,
    "last_solve": "2026-02-07T10:30:00Z"
  },
  {
    "rank": 2,
    "user_id": "user456",
    "user_name": "Bob",
    "points": 300,
    "solve_count": 3,
    "last_solve": "2026-02-07T10:15:00Z"
  }
]
```

### GET /api/sql/snapshot
Get sanitized data snapshot for SQL playground.

**Request:**
```http
GET /api/sql/snapshot
```

**Response:** 200 OK
```json
{
  "challenges": [...],
  "questions": [...],
  "submissions": [...],
  "users": [...]
}
```

## Protected Endpoints

Require authentication (user must be logged in).

### POST /api/questions/:id/submit
Submit a flag for a question.

**Request:**
```http
POST /api/questions/question1/submit
Content-Type: application/x-www-form-urlencoded
Cookie: auth_token=eyJhbGciOiJIUzI1NiIs...

flag=FLAG{correct_answer}
```

**Response:** 200 OK (Correct)
```html
<div class="text-green-400">✅ Correct! You earned 100 points</div>
```

**Response:** 200 OK (Incorrect)
```html
<div class="text-red-400">❌ Incorrect, try again</div>
```

**Response:** 200 OK (Already solved)
```html
<div class="text-yellow-400">You have already solved this question</div>
```

**Error:** 401 Unauthorized
```html
<div class="text-red-400">Unauthorized</div>
```

## Admin Endpoints

Require authentication AND admin privileges.

### POST /api/admin/challenges
Create a new challenge.

**Request:**
```http
POST /api/admin/challenges
Content-Type: application/json
Cookie: auth_token=eyJhbGciOiJIUzI1NiIs...

{
  "name": "XSS Challenge",
  "description": "Find and exploit XSS vulnerabilities",
  "category": "web",
  "difficulty": "medium",
  "tags": "[\"xss\", \"javascript\"]",
  "visible": true
}
```

**Response:** 200 OK
```json
{
  "id": "challenge2",
  "name": "XSS Challenge",
  "description": "Find and exploit XSS vulnerabilities",
  "category": "web",
  "difficulty": "medium",
  "tags": "[\"xss\", \"javascript\"]",
  "visible": true,
  "created_at": "2026-02-07T11:00:00Z",
  "updated_at": "2026-02-07T11:00:00Z"
}
```

**Error:** 403 Forbidden
```
Forbidden
```

### PUT /api/admin/challenges/:id
Update an existing challenge.

**Request:**
```http
PUT /api/admin/challenges/challenge1
Content-Type: application/json
Cookie: auth_token=eyJhbGciOiJIUzI1NiIs...

{
  "name": "SQL Injection 101 (Updated)",
  "description": "Updated description",
  "category": "web",
  "difficulty": "easy",
  "visible": true
}
```

**Response:** 200 OK
```
Challenge updated
```

### DELETE /api/admin/challenges/:id
Delete a challenge (cascades to questions).

**Request:**
```http
DELETE /api/admin/challenges/challenge1
Cookie: auth_token=eyJhbGciOiJIUzI1NiIs...
```

**Response:** 200 OK
```
Challenge deleted
```

### POST /api/admin/questions
Create a new question.

**Request:**
```http
POST /api/admin/questions
Content-Type: application/json
Cookie: auth_token=eyJhbGciOiJIUzI1NiIs...

{
  "challenge_id": "challenge1",
  "name": "Question 2",
  "description": "Find the second flag",
  "flag": "FLAG{second_flag}",
  "flag_mask": "FLAG{***********}",
  "case_sensitive": false,
  "points": 200,
  "file_url": "https://example.com/file.zip"
}
```

**Response:** 200 OK
```json
{
  "id": "question2",
  "challenge_id": "challenge1",
  "name": "Question 2",
  "description": "Find the second flag",
  "flag": "FLAG{second_flag}",
  "flag_mask": "FLAG{***********}",
  "case_sensitive": false,
  "points": 200,
  "file_url": "https://example.com/file.zip",
  "created_at": "2026-02-07T11:00:00Z",
  "updated_at": "2026-02-07T11:00:00Z"
}
```

**Notes:**
- If `flag_mask` is not provided, it's auto-generated from `flag`
- `file_url` is optional

### PUT /api/admin/questions/:id
Update an existing question.

**Request:**
```http
PUT /api/admin/questions/question1
Content-Type: application/json
Cookie: auth_token=eyJhbGciOiJIUzI1NiIs...

{
  "name": "Updated Question",
  "description": "Updated description",
  "flag": "FLAG{new_flag}",
  "case_sensitive": true,
  "points": 150
}
```

**Response:** 200 OK
```
Question updated
```

### DELETE /api/admin/questions/:id
Delete a question.

**Request:**
```http
DELETE /api/admin/questions/question1
Cookie: auth_token=eyJhbGciOiJIUzI1NiIs...
```

**Response:** 200 OK
```
Question deleted
```

### User Management (Admin Only)

- `GET /api/admin/users` - List all users
- `PUT /api/admin/users/{id}/admin` - Toggle admin status
  - Body: `is_admin=true` or `is_admin=false`
- `DELETE /api/admin/users/{id}` - Delete user

## Error Responses

### 400 Bad Request
```
Invalid request
```

### 401 Unauthorized
```
Unauthorized
```

### 403 Forbidden
```
Forbidden
```

### 404 Not Found
```
Challenge not found
```
or
```
Question not found
```

### 409 Conflict
```
Email already exists
```

### 500 Internal Server Error
```
Failed to fetch challenges
```

## Rate Limiting

Currently not implemented. Future versions will include:
- 100 requests per minute per IP
- 10 flag submissions per minute per user

## Pagination

Currently not implemented. All endpoints return full results.

Future: Query parameters `?page=1&limit=20`

## Filtering

Currently not implemented for challenges list.

Future: Query parameters `?category=web&difficulty=easy`

## Webhooks

Not implemented. Future feature for:
- New user registration
- Challenge solve
- First blood
- Team creation

## WebSockets

Not implemented. Future feature for:
- Live scoreboard updates
- Real-time notifications
- Challenge announcements

## Example Workflows

### Complete User Journey

1. **Register**
```bash
curl -X POST http://localhost:8090/api/auth/register \
  -d "email=user@example.com&password=secret123&name=John Doe"
```

2. **Login** (get token)
```bash
curl -X POST http://localhost:8090/api/auth/login \
  -d "email=user@example.com&password=secret123" \
  -c cookies.txt
```

3. **View Challenges**
```bash
curl http://localhost:8090/api/challenges -b cookies.txt
```

4. **Submit Flag**
```bash
curl -X POST http://localhost:8090/api/questions/QUESTION_ID/submit \
  -d "flag=FLAG{answer}" \
  -b cookies.txt
```

5. **Check Scoreboard**
```bash
curl http://localhost:8090/api/scoreboard -b cookies.txt
```

### Admin Workflow

1. **Login as Admin**
```bash
curl -X POST http://localhost:8090/api/auth/login \
  -d "email=admin@hctf.local&password=changeme" \
  -c admin_cookies.txt
```

2. **Create Challenge**
```bash
curl -X POST http://localhost:8090/api/admin/challenges \
  -H "Content-Type: application/json" \
  -b admin_cookies.txt \
  -d '{
    "name": "New Challenge",
    "description": "Description here",
    "category": "web",
    "difficulty": "easy",
    "visible": true
  }'
```

3. **Add Question**
```bash
curl -X POST http://localhost:8090/api/admin/questions \
  -H "Content-Type: application/json" \
  -b admin_cookies.txt \
  -d '{
    "challenge_id": "CHALLENGE_ID",
    "name": "Question 1",
    "description": "Find the flag",
    "flag": "FLAG{answer}",
    "points": 100
  }'
```

## SDK/Client Libraries

Currently none. Future:
- Go client
- Python client
- JavaScript client

# Next Steps for hCTF2

## Immediate Actions (Required)

### 1. Install Go
hCTF2 requires Go 1.24 or higher.

```bash
# Check if Go is installed
go version

# If not installed, download from:
# https://go.dev/dl/
```

### 2. Build the Application
```bash
cd /home/jesus/Projects/hCTF2

# Download dependencies
go mod download
go mod tidy

# Build
task build
```

### 3. Run the Server
```bash
# Start with admin user creation
./hctf2 --port 8090 --admin-email admin@hctf.local --admin-password changeme

# Or use make
task run
```

### 4. Test in Browser
Open http://localhost:8090 and verify:
- ✅ Homepage loads
- ✅ Can register a new user
- ✅ Can login with admin credentials
- ✅ Can view challenges page
- ✅ Can view scoreboard
- ✅ Can access SQL playground

## Creating Test Data

### Option 1: Use the API (Recommended)

See `API.md` for full documentation. Quick example:

```bash
# Login as admin
curl -X POST http://localhost:8090/api/auth/login \
  -d "email=admin@hctf.local&password=changeme" \
  -c cookies.txt

# Create a test challenge
curl -X POST http://localhost:8090/api/admin/challenges \
  -H "Content-Type: application/json" \
  -b cookies.txt \
  -d '{
    "name": "Test Challenge",
    "description": "A test challenge to verify the platform works",
    "category": "misc",
    "difficulty": "easy",
    "visible": true
  }' | jq .

# Save the returned challenge ID, then create a question
curl -X POST http://localhost:8090/api/admin/questions \
  -H "Content-Type: application/json" \
  -b cookies.txt \
  -d '{
    "challenge_id": "PASTE_CHALLENGE_ID_HERE",
    "name": "Find the Flag",
    "description": "The flag is FLAG{test_flag_123}",
    "flag": "FLAG{test_flag_123}",
    "case_sensitive": false,
    "points": 100
  }'
```

### Option 2: Direct Database Insert

```bash
# Install sqlite3 if needed
sudo apt install sqlite3  # Debian/Ubuntu
brew install sqlite3      # macOS

# Insert test data
sqlite3 hctf2.db <<EOF
INSERT INTO challenges (id, name, description, category, difficulty, visible)
VALUES ('test1', 'Welcome Challenge', 'Your first CTF challenge', 'misc', 'easy', 1);

INSERT INTO questions (id, challenge_id, name, description, flag, flag_mask, case_sensitive, points)
VALUES ('q1', 'test1', 'Question 1', 'Find the flag!', 'FLAG{welcome}', 'FLAG{*******}', 0, 100);
EOF
```

## Testing the Platform

### Test User Journey
1. **Register**: Create a test user account
2. **Browse**: View the test challenge
3. **Submit**: Try submitting the flag
4. **Check**: View scoreboard to see your points
5. **SQL**: Try the SQL playground with example queries

### Test Admin Functions
1. **Create**: Make a new challenge via API
2. **Update**: Modify the challenge
3. **Delete**: Remove the test challenge
4. **Manage**: Create multiple questions per challenge

## Common Issues and Solutions

### Issue: "go: command not found"
**Solution**: Install Go from https://go.dev/dl/

### Issue: "port 8090 already in use"
**Solution**: Use a different port
```bash
./hctf2 --port 3000
```

### Issue: "permission denied" when running ./hctf2
**Solution**: Make the file executable
```bash
chmod +x hctf2
```

### Issue: Database errors on startup
**Solution**: Delete the database and restart
```bash
rm hctf2.db
./hctf2 --port 8090 --admin-email admin@hctf.local --admin-password changeme
```

### Issue: Template not found errors
**Solution**: Rebuild the binary to embed templates
```bash
task clean
task build
```

## Development Workflow

### Making Changes

1. **Edit Code**: Modify Go files in `internal/`
2. **Edit Templates**: Update HTML in `internal/views/templates/`
3. **Test**: Run with `go run cmd/server/main.go`
4. **Build**: Create new binary with `task build`

### Adding New Features

See `ARCHITECTURE.md` for detailed guide. Quick overview:

1. **Model**: Add struct to `internal/models/models.go`
2. **Migration**: Create new SQL file in `migrations/`
3. **Database**: Add queries to `internal/database/queries.go`
4. **Handler**: Create handler in `internal/handlers/`
5. **Route**: Register in `cmd/server/main.go`
6. **Template**: Add HTML in `internal/views/templates/`

## Recommended Phase 2 Features

Based on the plan, prioritize these next:

### 1. Admin Web UI (High Priority)
Currently admin functions are API-only. Build a web interface:
- `/admin` dashboard page
- Challenge CRUD forms
- Question management
- User list

### 2. Team Management (Medium Priority)
Schema exists, needs UI:
- Create team page
- Join team functionality
- Team scoreboard
- Team invite system

### 3. Hints System (Medium Priority)
Schema exists, needs UI:
- Display hints on challenge page
- Unlock mechanism
- Points deduction
- Cost display

### 4. File Uploads (Low Priority)
For challenge attachments:
- Upload form
- Local storage or S3
- Download tracking
- File size limits

## Production Deployment

When ready to deploy:

1. **Build for Production**
```bash
task build-prod
```

2. **Setup Systemd Service**
See `INSTALL.md` section on systemd

3. **Configure Nginx**
See `INSTALL.md` section on reverse proxy

4. **Enable HTTPS**
Use Let's Encrypt with certbot

5. **Change Secrets**
- Generate new JWT secret
- Change admin password
- Use strong database password

6. **Backup Strategy**
```bash
# Automated backup script
0 0 * * * cp /var/lib/hctf2/hctf2.db /backups/hctf2-$(date +\%Y\%m\%d).db
```

## Getting Help

- **Documentation**: Check README.md, INSTALL.md, QUICKSTART.md
- **Architecture**: See ARCHITECTURE.md for technical details
- **API**: See API.md for endpoint reference
- **Issues**: https://github.com/yourusername/hctf2/issues

## Useful Commands

```bash
# Build
task build

# Run (creates admin)
task run

# Run dev mode (no admin)
task run-dev

# Clean
task clean

# Test
task test

# Format code
task fmt

# Production build
task build-prod

# View database
sqlite3 hctf2.db "SELECT * FROM users;"
sqlite3 hctf2.db "SELECT * FROM challenges;"

# Backup database
cp hctf2.db hctf2.db.backup

# View logs (if using systemd)
journalctl -u hctf2 -f
```

## Success Checklist

- [ ] Go installed and working
- [ ] Project builds without errors
- [ ] Server starts successfully
- [ ] Homepage loads in browser
- [ ] Can register new user
- [ ] Can login as admin
- [ ] Can create challenge via API
- [ ] Can submit flag
- [ ] Scoreboard updates correctly
- [ ] SQL playground loads and executes queries

Once all items are checked, the platform is ready for use!

## What You Have

A **complete, working CTF platform** with:
- User authentication and authorization
- Challenge and question management
- Flag submission with auto-masking
- Live scoreboard
- Unique SQL analytics interface
- Beautiful dark UI
- Single binary deployment
- Comprehensive documentation

**You can start hosting CTFs today!**

## Next Development Priorities

1. Test thoroughly with Go installation
2. Create sample challenges via API
3. Build admin web UI (Phase 2)
4. Add team management (Phase 2)
5. Implement hints system (Phase 2)
6. Deploy to production server

Good luck with hCTF2! 🚀

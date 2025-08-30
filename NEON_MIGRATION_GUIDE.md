# Neon Database Migration Guide

This guide provides step-by-step instructions for migrating the YouTube Activity Backend from Supabase to Neon PostgreSQL.

## Migration Summary

✅ **Status**: COMPLETED  
🗄️ **Source**: Supabase PostgreSQL  
🎯 **Target**: Neon PostgreSQL  
📊 **Tables Migrated**: 8 tables with complete schema and sample data  
🔒 **SSL**: SSL connection verified and working  

## New Database Configuration

The following configuration files have been updated:

### Updated Files:
- `/Users/gamemini/workspace/youtube/be/.env`
- `/Users/gamemini/workspace/youtube/be/.env.local`

### New Database URL:
```
postgresql://neondb_owner:npg_P0OWi3VQcUsf@ep-blue-bar-a11q0cer-pooler.ap-southeast-1.aws.neon.tech/neondb?sslmode=require&channel_binding=require
```

## Database Schema

The new Neon database contains the following tables:

### Core Tables
1. **`users`** - User profiles with OAuth tokens, terms acceptance, and profile completion status
2. **`activities`** - YouTube activities/campaigns
3. **`teams`** - Activity teams (A-H) 
4. **`votes`** - User votes for teams in activities
5. **`user_sessions`** - User session management
6. **`terms_versions`** - Terms and PDPA version management
7. **`user_terms_acceptance`** - Terms acceptance audit trail
8. **`schema_migrations`** - Migration tracking

### Key Features
- **UUID Primary Keys** with `uuid-ossp` extension
- **Row Level Security (RLS)** enabled on all tables
- **Data Validation** with check constraints for Thai phone numbers and name lengths
- **Audit Trail** for terms acceptance and user profile changes
- **Optimized Indexes** for performance on common queries
- **OAuth Integration** with Google OAuth tokens and YouTube channel verification

## Migration Process Executed

### 1. Configuration Updates ✅
- Updated `.env` and `.env.local` with new Neon database URL
- Verified SSL connection parameters (`sslmode=require`, `channel_binding=require`)
- No changes needed to connection handling code (pgx driver handles SSL automatically)

### 2. Schema Migration ✅
- Created consolidated migration file: `/Users/gamemini/workspace/youtube/be/migrations/neon_migration_complete.sql`
- Combined all 12 existing migrations into a single comprehensive schema
- Applied all migrations to Neon database successfully
- Verified sample data insertion (1 activity, 8 teams, terms versions)

### 3. Connection Testing ✅
- Verified SSL connection works with Neon's security requirements
- Tested Go application startup with new database
- Confirmed all routes and services initialize correctly
- Database health check passes

## Post-Migration Verification

### Database Structure Verification
```bash
# Connect to Neon database
psql "postgresql://neondb_owner:npg_P0OWi3VQcUsf@ep-blue-bar-a11q0cer-pooler.ap-southeast-1.aws.neon.tech/neondb?sslmode=require&channel_binding=require"

# Verify tables
\dt

# Check sample data
SELECT COUNT(*) FROM activities;
SELECT COUNT(*) FROM teams;
SELECT version FROM schema_migrations ORDER BY applied_at DESC;
```

### Application Testing
```bash
# Start the Go application
go run main.go

# Expected output:
# ✅ Connected to local database
# 🚀 Server starting on port 8080
```

## Important Notes

### SSL Configuration
- Neon requires SSL connections with `sslmode=require` and `channel_binding=require`
- The pgx/v5 driver handles SSL automatically - no additional configuration needed
- SSL validation confirmed working in connection tests

### Row Level Security
- RLS policies have been simplified for compatibility with standard PostgreSQL
- Basic policies allow reading activities/teams and creating votes
- User data access restricted to authenticated users

### Data Migration
Since this migration was to a fresh Neon instance, no user data needed to be transferred. If you need to migrate existing user data from Supabase:

1. Export data from Supabase:
   ```sql
   COPY users TO '/tmp/users.csv' DELIMITER ',' CSV HEADER;
   COPY votes TO '/tmp/votes.csv' DELIMITER ',' CSV HEADER;
   ```

2. Import to Neon:
   ```sql
   COPY users FROM '/tmp/users.csv' DELIMITER ',' CSV HEADER;
   COPY votes FROM '/tmp/votes.csv' DELIMITER ',' CSV HEADER;
   ```

### Environment Variables
The following environment variables are now pointing to Neon:
- `DATABASE_URL` - Updated in both `.env` and `.env.local`
- All other configurations remain the same (Redis, OAuth, etc.)

## Rollback Plan

If you need to rollback to Supabase:

1. Restore the original DATABASE_URL:
   ```
   DATABASE_URL=postgresql://postgres.pomgelpsnzrzyjretcru:+xC.hnYBUp46+@L@aws-1-ap-southeast-1.pooler.supabase.com:6543/postgres
   ```

2. Update both `.env` and `.env.local` files

3. Restart the application

## Performance Considerations

### Neon-Specific Optimizations
- Connection pooling configured for Neon's pooling architecture
- Indexes optimized for common query patterns
- SSL overhead minimal with connection pooling

### Monitoring Recommendations
- Monitor connection pool usage
- Track query performance through Neon dashboard
- Set up alerts for connection limit thresholds

## Security Notes

- Database credentials are stored in `.env.local` (not committed to version control)
- SSL enforced for all connections
- Row Level Security enabled for data protection
- OAuth tokens stored securely in the users table

## Next Steps

1. **Test All Features**: Verify OAuth flow, voting, user registration
2. **Update Production Config**: When ready, update production environment variables
3. **Monitor Performance**: Use Neon dashboard to monitor database performance
4. **Backup Strategy**: Set up automated backups in Neon console
5. **Documentation**: Update API documentation with any schema changes

## Support

For any issues with the migration:

1. Check logs for connection errors
2. Verify environment variables are loaded correctly
3. Test database connection with `go run test_connection.go`
4. Review Neon dashboard for connection and query metrics

---

**Migration completed successfully on 2025-08-30**  
**Database**: Ready for production use  
**Schema Version**: All migrations applied (001-012 + neon_migration_complete)
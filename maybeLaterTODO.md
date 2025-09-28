# Maybe Later TODO

## PERFORMANCE

### Database Connection Management
- Add connection pooling with proper limits
- Configure MaxOpenConns, MaxIdleConns, ConnMaxLifetime
- **Reason**: Not needed for single user, database mostly idling
- **Impact**: Low effort, but unnecessary for current scale

### WebSocket Client Management
- Redis for client state management
- Horizontal scaling with Redis pub/sub
- **Reason**: No scaling planned, single server sufficient
- **Impact**: Unnecessary complexity for current needs

### Database Indexes
```sql
-- Recommended indexes for better performance
CREATE INDEX idx_tasks_user_id_active ON tasks(user_id, is_completed) WHERE is_completed = FALSE;
CREATE INDEX idx_tasks_user_id_completed ON tasks(user_id, completed_at) WHERE is_completed = TRUE;
CREATE INDEX idx_tasks_category ON tasks(category);
CREATE INDEX idx_tasks_tags ON tasks USING GIN(tags);
```
- **Reason**: 25k tasks over 2 years, queries mostly by UUID + is_active
- **Impact**: Low effort, but may not be necessary for current scale
- **Note**: Monitor query performance as data grows

### Memory Usage Optimization
- Lazy loading and pagination
- **Reason**: Need to verify if tasks are actually loaded in memory
- **Impact**: Depends on current implementation

### Caching Layer
- Redis for frequently accessed data
- Cache user settings, categories, etc.
- **Reason**: Single user, data access patterns don't justify caching
- **Impact**: Unnecessary complexity

### Database Read Replicas
- **Reason**: Single user, no read scaling needed
- **Impact**: Unnecessary infrastructure

### CDN for Static Assets
- **Reason**: Minimal static assets, single user
- **Impact**: Unnecessary complexity

## SCALABILITY

### Multi-tenancy
- Organization/team concept
- **Reason**: Single user, no team collaboration planned
- **Impact**: Unnecessary complexity

### Horizontal Scaling
- Load balancing
- Multiple server instances
- **Reason**: No scaling planned
- **Impact**: Unnecessary infrastructure

## MONITORING

### Prometheus Metrics
- Performance monitoring
- Query timing
- **Reason**: Single user, no complex monitoring needed
- **Impact**: Unnecessary complexity for current scale

### Advanced Logging
- Structured logging
- Performance metrics
- **Reason**: Basic logging sufficient for current needs
- **Impact**: Low priority

## SECURITY

### Advanced Authentication
- JWT tokens
- OAuth integration
- **Reason**: Google auth sufficient for personal use
- **Impact**: Unnecessary complexity for current scale

### Rate Limiting
- API rate limits
- **Reason**: Single user, no abuse risk
- **Impact**: Unnecessary complexity

## INFRASTRUCTURE

### Containerization
- Docker deployment
- **Reason**: Current deployment works fine
- **Impact**: Unnecessary complexity

### CI/CD Pipeline
- Automated deployment
- **Reason**: Manual deployment sufficient for personal use
- **Impact**: Unnecessary complexity

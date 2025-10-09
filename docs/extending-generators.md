# Extending Source Generators

This guide provides detailed instructions for implementing custom source generators to extend the ExternalSource Controller with new data source types.

## Generator Interface Overview

All source generators must implement the `SourceGenerator` interface:

```go
type SourceGenerator interface {
    Generate(ctx context.Context, config GeneratorConfig) (*SourceData, error)
    SupportsConditionalFetch() bool
    GetLastModified(ctx context.Context, config GeneratorConfig) (string, error)
}
```

## Step-by-Step Implementation Guide

### Step 1: Define Your Generator Configuration

First, define the configuration structure for your generator. This will be embedded in the ExternalSource CRD.

```go
// api/v1alpha1/externalsource_types.go

// Add to the Generator struct
type Generator struct {
    Type string `json:"type"`
    
    // Existing generators
    // +optional
    HTTP *HTTPGenerator `json:"http,omitempty"`
    
    // Your new generator
    // +optional
    Database *DatabaseGenerator `json:"database,omitempty"`
}

// Define your generator's configuration
type DatabaseGenerator struct {
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:Enum=mysql;postgresql;mongodb
    Type string `json:"type"`
    
    // +kubebuilder:validation:Required
    ConnectionSecretRef LocalObjectReference `json:"connectionSecretRef"`
    
    // +kubebuilder:validation:Required
    Query string `json:"query"`
    
    // +optional
    Timeout *metav1.Duration `json:"timeout,omitempty"`
    
    // +optional
    Parameters map[string]string `json:"parameters,omitempty"`
}
```

### Step 2: Implement the Generator

Create your generator implementation:

```go
// internal/generator/database.go
package generator

import (
    "context"
    "database/sql"
    "encoding/json"
    "fmt"
    "time"
    
    _ "github.com/go-sql-driver/mysql"
    _ "github.com/lib/pq"
    "go.mongodb.org/mongo-driver/mongo"
)

type DatabaseGenerator struct {
    secretClient SecretClient
    logger       logr.Logger
}

type DatabaseConfig struct {
    Type                string            `json:"type"`
    ConnectionSecretRef string            `json:"connectionSecretRef"`
    Query               string            `json:"query"`
    Timeout             time.Duration     `json:"timeout"`
    Parameters          map[string]string `json:"parameters"`
}

func NewDatabaseGenerator(secretClient SecretClient, logger logr.Logger) *DatabaseGenerator {
    return &DatabaseGenerator{
        secretClient: secretClient,
        logger:       logger,
    }
}

func (g *DatabaseGenerator) Generate(ctx context.Context, config GeneratorConfig) (*SourceData, error) {
    log := g.logger.WithValues("generator", "database")
    
    // Parse configuration
    dbConfig, err := g.parseConfig(config)
    if err != nil {
        return nil, fmt.Errorf("failed to parse database config: %w", err)
    }
    
    // Get connection string from secret
    connectionString, err := g.getConnectionString(ctx, dbConfig.ConnectionSecretRef)
    if err != nil {
        return nil, fmt.Errorf("failed to get connection string: %w", err)
    }
    
    // Execute query based on database type
    var data []byte
    var lastModified string
    
    switch dbConfig.Type {
    case "mysql", "postgresql":
        data, lastModified, err = g.executeSQL(ctx, dbConfig, connectionString)
    case "mongodb":
        data, lastModified, err = g.executeMongo(ctx, dbConfig, connectionString)
    default:
        return nil, fmt.Errorf("unsupported database type: %s", dbConfig.Type)
    }
    
    if err != nil {
        return nil, fmt.Errorf("failed to execute query: %w", err)
    }
    
    log.V(1).Info("Database query completed", "size", len(data))
    
    return &SourceData{
        Data:         data,
        LastModified: lastModified,
        Metadata: map[string]string{
            "database_type": dbConfig.Type,
            "query":         dbConfig.Query,
            "rows":          fmt.Sprintf("%d", len(data)),
        },
    }, nil
}

func (g *DatabaseGenerator) SupportsConditionalFetch() bool {
    // Database queries typically don't support conditional fetching
    // unless you implement custom logic with timestamps or version columns
    return false
}

func (g *DatabaseGenerator) GetLastModified(ctx context.Context, config GeneratorConfig) (string, error) {
    // Not supported for basic database queries
    return "", nil
}

func (g *DatabaseGenerator) parseConfig(config GeneratorConfig) (*DatabaseConfig, error) {
    configBytes, err := json.Marshal(config.Config)
    if err != nil {
        return nil, err
    }
    
    var dbConfig DatabaseConfig
    if err := json.Unmarshal(configBytes, &dbConfig); err != nil {
        return nil, err
    }
    
    // Set defaults
    if dbConfig.Timeout == 0 {
        dbConfig.Timeout = 30 * time.Second
    }
    
    // Validate required fields
    if dbConfig.Type == "" {
        return nil, fmt.Errorf("database type is required")
    }
    if dbConfig.ConnectionSecretRef == "" {
        return nil, fmt.Errorf("connectionSecretRef is required")
    }
    if dbConfig.Query == "" {
        return nil, fmt.Errorf("query is required")
    }
    
    return &dbConfig, nil
}

func (g *DatabaseGenerator) executeSQL(ctx context.Context, config *DatabaseConfig, connectionString string) ([]byte, string, error) {
    // Create context with timeout
    ctx, cancel := context.WithTimeout(ctx, config.Timeout)
    defer cancel()
    
    // Open database connection
    db, err := sql.Open(config.Type, connectionString)
    if err != nil {
        return nil, "", fmt.Errorf("failed to open database: %w", err)
    }
    defer db.Close()
    
    // Test connection
    if err := db.PingContext(ctx); err != nil {
        return nil, "", fmt.Errorf("failed to ping database: %w", err)
    }
    
    // Prepare query with parameters
    query := config.Query
    var args []interface{}
    
    for key, value := range config.Parameters {
        // Simple parameter substitution (in production, use proper parameter binding)
        query = strings.ReplaceAll(query, fmt.Sprintf("${%s}", key), value)
    }
    
    // Execute query
    rows, err := db.QueryContext(ctx, query, args...)
    if err != nil {
        return nil, "", fmt.Errorf("failed to execute query: %w", err)
    }
    defer rows.Close()
    
    // Get column names
    columns, err := rows.Columns()
    if err != nil {
        return nil, "", fmt.Errorf("failed to get columns: %w", err)
    }
    
    // Collect results
    var results []map[string]interface{}
    
    for rows.Next() {
        // Create slice for row values
        values := make([]interface{}, len(columns))
        valuePtrs := make([]interface{}, len(columns))
        
        for i := range values {
            valuePtrs[i] = &values[i]
        }
        
        // Scan row
        if err := rows.Scan(valuePtrs...); err != nil {
            return nil, "", fmt.Errorf("failed to scan row: %w", err)
        }
        
        // Create result map
        result := make(map[string]interface{})
        for i, col := range columns {
            result[col] = values[i]
        }
        
        results = append(results, result)
    }
    
    if err := rows.Err(); err != nil {
        return nil, "", fmt.Errorf("row iteration error: %w", err)
    }
    
    // Marshal to JSON
    data, err := json.Marshal(results)
    if err != nil {
        return nil, "", fmt.Errorf("failed to marshal results: %w", err)
    }
    
    // Generate last modified timestamp
    lastModified := time.Now().UTC().Format(time.RFC3339)
    
    return data, lastModified, nil
}

func (g *DatabaseGenerator) executeMongo(ctx context.Context, config *DatabaseConfig, connectionString string) ([]byte, string, error) {
    // MongoDB implementation
    // This is a simplified example - in practice, you'd need proper MongoDB query parsing
    
    client, err := mongo.Connect(ctx, options.Client().ApplyURI(connectionString))
    if err != nil {
        return nil, "", fmt.Errorf("failed to connect to MongoDB: %w", err)
    }
    defer client.Disconnect(ctx)
    
    // Parse database and collection from query
    // This is a simplified parser - implement proper MongoDB query parsing
    parts := strings.Split(config.Query, ".")
    if len(parts) < 2 {
        return nil, "", fmt.Errorf("invalid MongoDB query format")
    }
    
    database := parts[0]
    collection := parts[1]
    
    coll := client.Database(database).Collection(collection)
    
    // Execute find operation
    cursor, err := coll.Find(ctx, bson.M{})
    if err != nil {
        return nil, "", fmt.Errorf("failed to execute MongoDB query: %w", err)
    }
    defer cursor.Close(ctx)
    
    // Collect results
    var results []bson.M
    if err := cursor.All(ctx, &results); err != nil {
        return nil, "", fmt.Errorf("failed to decode MongoDB results: %w", err)
    }
    
    // Marshal to JSON
    data, err := json.Marshal(results)
    if err != nil {
        return nil, "", fmt.Errorf("failed to marshal MongoDB results: %w", err)
    }
    
    lastModified := time.Now().UTC().Format(time.RFC3339)
    
    return data, lastModified, nil
}

func (g *DatabaseGenerator) getConnectionString(ctx context.Context, secretRef string) (string, error) {
    secret, err := g.secretClient.GetSecret(ctx, secretRef)
    if err != nil {
        return "", fmt.Errorf("failed to get secret %s: %w", secretRef, err)
    }
    
    connectionString, exists := secret.Data["connectionString"]
    if !exists {
        return "", fmt.Errorf("connectionString not found in secret %s", secretRef)
    }
    
    return string(connectionString), nil
}

// SecretClient interface for dependency injection
type SecretClient interface {
    GetSecret(ctx context.Context, name string) (*corev1.Secret, error)
}
```

### Step 3: Add CRD Validation

Update the CRD schema to include validation for your new generator:

```go
// Update the enum validation
// +kubebuilder:validation:Enum=http;database
Type string `json:"type"`
```

Regenerate the CRDs:

```bash
make manifests
```

### Step 4: Register the Generator

Register your generator in the controller:

```go
// internal/controller/externalsource_controller.go

func (r *ExternalSourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
    // Initialize factory
    r.factory = generator.NewFactory()
    
    // Register existing generators
    r.factory.RegisterGenerator("http", func() generator.SourceGenerator {
        return generator.NewHTTPGenerator(r.httpClient)
    })
    
    // Register your new generator
    r.factory.RegisterGenerator("database", func() generator.SourceGenerator {
        return generator.NewDatabaseGenerator(r.secretClient, r.Log)
    })
    
    return ctrl.NewControllerManagedBy(mgr).
        For(&sourcev1alpha1.ExternalSource{}).
        Complete(r)
}
```

### Step 5: Add Comprehensive Tests

Create thorough tests for your generator:

```go
// internal/generator/database_test.go
package generator

import (
    "context"
    "testing"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    corev1 "k8s.io/api/core/v1"
)

type MockSecretClient struct {
    mock.Mock
}

func (m *MockSecretClient) GetSecret(ctx context.Context, name string) (*corev1.Secret, error) {
    args := m.Called(ctx, name)
    return args.Get(0).(*corev1.Secret), args.Error(1)
}

func TestDatabaseGenerator_Generate(t *testing.T) {
    tests := []struct {
        name          string
        config        GeneratorConfig
        secretSetup   func(*MockSecretClient)
        expectedError string
        expectedData  string
    }{
        {
            name: "successful MySQL query",
            config: GeneratorConfig{
                Type: "database",
                Config: map[string]interface{}{
                    "type":                "mysql",
                    "connectionSecretRef": "db-secret",
                    "query":               "SELECT * FROM users",
                },
            },
            secretSetup: func(m *MockSecretClient) {
                secret := &corev1.Secret{
                    Data: map[string][]byte{
                        "connectionString": []byte("user:pass@tcp(localhost:3306)/testdb"),
                    },
                }
                m.On("GetSecret", mock.Anything, "db-secret").Return(secret, nil)
            },
            // Note: This test would require a real database or more sophisticated mocking
        },
        {
            name: "missing connection secret",
            config: GeneratorConfig{
                Type: "database",
                Config: map[string]interface{}{
                    "type":                "mysql",
                    "connectionSecretRef": "missing-secret",
                    "query":               "SELECT * FROM users",
                },
            },
            secretSetup: func(m *MockSecretClient) {
                m.On("GetSecret", mock.Anything, "missing-secret").
                    Return((*corev1.Secret)(nil), fmt.Errorf("secret not found"))
            },
            expectedError: "failed to get connection string",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            mockSecretClient := &MockSecretClient{}
            if tt.secretSetup != nil {
                tt.secretSetup(mockSecretClient)
            }
            
            generator := NewDatabaseGenerator(mockSecretClient, logr.Discard())
            
            result, err := generator.Generate(context.Background(), tt.config)
            
            if tt.expectedError != "" {
                assert.Error(t, err)
                assert.Contains(t, err.Error(), tt.expectedError)
                return
            }
            
            assert.NoError(t, err)
            assert.NotNil(t, result)
            mockSecretClient.AssertExpectations(t)
        })
    }
}
```

### Step 6: Create Examples and Documentation

Create example configurations:

```yaml
# examples/database-source.yaml
apiVersion: v1
kind: Secret
metadata:
  name: mysql-connection
  namespace: default
type: Opaque
data:
  # Base64 encoded connection string
  connectionString: dXNlcjpwYXNzd29yZEB0Y3AobXlzcWw6MzMwNikvbXlkYXRhYmFzZQ==

---
apiVersion: source.example.com/v1alpha1
kind: ExternalSource
metadata:
  name: user-data-source
  namespace: default
spec:
  interval: 15m
  destinationPath: users.json
  generator:
    type: database
    database:
      type: mysql
      connectionSecretRef:
        name: mysql-connection
      query: |
        SELECT 
          id, 
          username, 
          email, 
          created_at 
        FROM users 
        WHERE active = 1 
        ORDER BY created_at DESC
      timeout: 30s
      parameters:
        limit: "100"

---
# Example with transformation
apiVersion: source.example.com/v1alpha1
kind: ExternalSource
metadata:
  name: config-from-db
  namespace: default
spec:
  interval: 10m
  destinationPath: app-config.yaml
  generator:
    type: database
    database:
      type: postgresql
      connectionSecretRef:
        name: postgres-connection
      query: |
        SELECT key, value, environment 
        FROM app_config 
        WHERE environment = 'production'
  transform:
    type: cel
    expression: |
      {
        "apiVersion": "v1",
        "kind": "ConfigMap",
        "metadata": {"name": "app-config"},
        "data": data.fold({}, (acc, item) => 
          acc + {item.key: item.value}
        )
      }
```

## Advanced Generator Features

### Conditional Fetching Support

If your data source supports conditional fetching (like ETags or timestamps):

```go
func (g *DatabaseGenerator) SupportsConditionalFetch() bool {
    return true
}

func (g *DatabaseGenerator) GetLastModified(ctx context.Context, config GeneratorConfig) (string, error) {
    // Query for the latest modification timestamp
    query := "SELECT MAX(updated_at) FROM " + config.TableName
    
    var lastModified time.Time
    err := g.db.QueryRowContext(ctx, query).Scan(&lastModified)
    if err != nil {
        return "", err
    }
    
    return lastModified.Format(time.RFC3339), nil
}
```

### Streaming Support for Large Datasets

For large datasets, implement streaming:

```go
func (g *DatabaseGenerator) GenerateStream(ctx context.Context, config GeneratorConfig) (<-chan []byte, <-chan error) {
    dataChan := make(chan []byte, 100)
    errorChan := make(chan error, 1)
    
    go func() {
        defer close(dataChan)
        defer close(errorChan)
        
        // Stream results in chunks
        rows, err := g.db.QueryContext(ctx, config.Query)
        if err != nil {
            errorChan <- err
            return
        }
        defer rows.Close()
        
        var batch []map[string]interface{}
        batchSize := 1000
        
        for rows.Next() {
            // Process row...
            batch = append(batch, row)
            
            if len(batch) >= batchSize {
                data, _ := json.Marshal(batch)
                dataChan <- data
                batch = batch[:0] // Reset batch
            }
        }
        
        // Send remaining batch
        if len(batch) > 0 {
            data, _ := json.Marshal(batch)
            dataChan <- data
        }
    }()
    
    return dataChan, errorChan
}
```

### Metrics and Observability

Add metrics for your generator:

```go
var (
    databaseQueryDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "externalsource_database_query_duration_seconds",
            Help: "Duration of database queries",
        },
        []string{"database_type", "status"},
    )
    
    databaseQueryTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "externalsource_database_query_total",
            Help: "Total number of database queries",
        },
        []string{"database_type", "status"},
    )
)

func (g *DatabaseGenerator) Generate(ctx context.Context, config GeneratorConfig) (*SourceData, error) {
    start := time.Now()
    
    defer func() {
        duration := time.Since(start).Seconds()
        databaseQueryDuration.WithLabelValues(config.Type, "success").Observe(duration)
        databaseQueryTotal.WithLabelValues(config.Type, "success").Inc()
    }()
    
    // Implementation...
}
```

## Best Practices

### Error Handling

- Provide detailed error messages with context
- Distinguish between transient and permanent errors
- Use structured logging for debugging

### Security

- Never log sensitive data (passwords, tokens)
- Validate all input parameters
- Use secure connection methods (TLS)
- Implement proper timeout handling

### Performance

- Implement connection pooling for database generators
- Add appropriate timeouts for all operations
- Consider implementing caching for frequently accessed data
- Use streaming for large datasets

### Testing

- Mock all external dependencies
- Test error scenarios thoroughly
- Include integration tests with real services
- Validate configuration parsing and validation

This guide provides a comprehensive foundation for implementing custom source generators. The modular architecture makes it straightforward to add new data sources while maintaining consistency and reliability.
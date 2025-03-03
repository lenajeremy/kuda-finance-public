package graph

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

type Conn struct {
	driver neo4j.DriverWithContext
}

func NewGraphConn() (*Conn, error) {
	var (
		Neo4jUrl  = os.Getenv("GO_NEO4J_URI")
		Neo4jUser = os.Getenv("GO_NEO4J_USERNAME")
		Neo4jPass = os.Getenv("GO_NEO4J_PASSWORD")
	)

	slog.Info(Neo4jUrl, Neo4jPass, Neo4jUser)

	driver, err := neo4j.NewDriverWithContext(
		Neo4jUrl,
		neo4j.BasicAuth(Neo4jUser, Neo4jPass, ""),
	)

	if err != nil {
		return nil, err
	}

	err = driver.VerifyConnectivity(context.Background())
	if err != nil {
		return nil, err
	}

	return &Conn{driver: driver}, nil
}

func (g *Conn) Execute(ctx context.Context, query string, params map[string]interface{}) (*neo4j.EagerResult, error) {
	res, err := neo4j.ExecuteQuery(
		ctx, g.driver,
		query, params,
		neo4j.EagerResultTransformer,
		neo4j.ExecuteQueryWithDatabase("neo4j"),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to execute query. CYPHER: %s. Error: %s", query, err.Error())
	}

	return res, nil
}

func (g *Conn) Close() error {
	return g.driver.Close(context.Background())
}

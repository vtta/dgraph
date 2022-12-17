package main

import (
	"context"
	// "encoding/json"
	"fmt"
	"log"
	// "strings"
	// "time"

	"github.com/dgraph-io/dgo/v210"
	"github.com/dgraph-io/dgo/v210/protos/api"
	"google.golang.org/grpc"
)

func main() {
	conn, err := grpc.Dial("localhost:9080", grpc.WithInsecure())
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	dgraphClient := dgo.NewDgraphClient(api.NewDgraphClient(conn))
	ctx := context.Background()
	op := &api.Operation{
		Schema: `name: string @index(exact) .`,
	}

	err = dgraphClient.Alter(ctx, op)
	fmt.Println(err)
}

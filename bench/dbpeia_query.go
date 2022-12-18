package main

import (
	"context"
	// "fmt"
	"github.com/dgraph-io/dgo/v210"
	"github.com/dgraph-io/dgo/v210/protos/api"
	"google.golang.org/grpc"
	"log"
)

func run_job(cli *dgo.Dgraph, jobs int) {
	q := `{ me(func: has(<http://zh.dbpedia.org/property/姓名>))
            { <http://zh.dbpedia.org/property/姓名> }
        }`
	resps := make(chan *api.Response, jobs)
	ctx := context.Background()
	txn := cli.NewTxn()
	for i := 0; i < jobs; i++ {
		go func() {
			res, err := txn.Query(ctx, q)
			if err != nil {
				log.Println(err)
			}
			resps <- res
		}()
	}
	go func() {

		for i := 0; i < jobs; i++ {
			_ = <-resps
		}
		txn.Discard(ctx)
	}()
}

func main() {
	conn, err := grpc.Dial("localhost:9080", grpc.WithInsecure())
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	cli := dgo.NewDgraphClient(api.NewDgraphClient(conn))
	batches := 1 << 20
	batchsz := 100
	for i := 0; i < batches; i++ {
		run_job(cli, batchsz)
		// fmt.Printf("batch %s\n", "done")
	}
}

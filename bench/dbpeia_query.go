package main

import (
	"context"
	"fmt"
	"log"

	"github.com/dgraph-io/dgo/v210"
	"github.com/dgraph-io/dgo/v210/protos/api"
	"google.golang.org/grpc"
)

func run_job(cli *dgo.Dgraph, jobs int, fin chan bool) {
	q := `{ me(func: has(<http://zh.dbpedia.org/property/姓名>))
            { <http://zh.dbpedia.org/property/姓名> }
        }`
	ctx := context.Background()
	txn := cli.NewTxn()
	for i := 0; i < jobs; i++ {
		_, err := txn.Query(ctx, q)
		if err != nil {
			log.Fatalln(err)
		}
	}
	txn.Discard(ctx)
	fin <- true
}

func bench(shard int, fin chan bool) {
	addr := fmt.Sprintf("localhost:%v", 9080+shard)
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	defer conn.Close()
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	cli := dgo.NewDgraphClient(api.NewDgraphClient(conn))
	batches := 4
	batchsz := 128
	resps := make(chan bool, batches)
	for i := 0; i < batches; i++ {
		go run_job(cli, batchsz, resps)
	}
	for i := 0; i < batches; i++ {
		_ = <-resps
		// fmt.Printf("batch %s\n", "done")
	}
	fin <- true
}

func main() {
	shards := 3
	fin := make(chan bool, shards)
	for i := 0; i < shards; i++ {
		bench(i, fin)
	}
	for i := 0; i < shards; i++ {
		_ = <-fin
	}

}

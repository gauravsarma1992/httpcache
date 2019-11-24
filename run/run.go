package main

import (
	"httpcache"
	"log"
	"os"
)

func main() {

	var (
		httpCacheCtxt *httpcache.HttpCacheCtxt
		err           error
	)

	log.Println("Starting HttpCache Service")

	if httpCacheCtxt, err = httpcache.NewHttpCacheCtxt(); err != nil {
		log.Println(err)
		os.Exit(-1)
	}

	if err = httpCacheCtxt.Process(); err != nil {
		log.Println(err)
		os.Exit(-1)
	}

}

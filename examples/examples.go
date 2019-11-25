package main

import (
	"httpcache"
	"log"
	"net/http"
	"os"
)

func someHandler(w http.ResponseWriter,
	req *http.Request) (respBody []byte, err error) {

	respBody = []byte("{\"something\": \"some-response\"}")

	return
}

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

	if err = httpCacheCtxt.RegisterLocalCacheHandler("/something",
		someHandler); err != nil {

		log.Println(err)
		os.Exit(-1)
	}

	if err = httpCacheCtxt.Process(); err != nil {
		log.Println(err)
		os.Exit(-1)
	}

}

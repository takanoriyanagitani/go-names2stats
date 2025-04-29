package main

import (
	"context"
	"fmt"
	"iter"
	"log"
	"os"

	ns "github.com/takanoriyanagitani/go-names2stats"
	. "github.com/takanoriyanagitani/go-names2stats/util"
)

var envValByKey func(string) IO[string] = Lift(
	func(key string) (string, error) {
		val, found := os.LookupEnv(key)
		switch found {
		case true:
			return val, nil
		default:
			return "", fmt.Errorf("env var %s missing", key)
		}
	},
)

var rootDirname IO[string] = envValByKey("ENV_ROOT_DIR_NAME")

var rdir IO[ns.RootDirname] = Bind(
	rootDirname,
	Lift(func(s string) (ns.RootDirname, error) {
		return ns.RootDirname(s), nil
	}),
)

var filenames iter.Seq[string] = ns.StdinToNames()

var names2stats2jsonl2stdout IO[Void] = Bind(
	rdir,
	Lift(func(r ns.RootDirname) (Void, error) {
		return Empty, r.NamesToBasicStatsToStdout(filenames)
	}),
)

func main() {
	_, e := names2stats2jsonl2stdout(context.Background())
	if nil != e {
		log.Printf("%v\n", e)
	}
}

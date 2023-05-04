package gojamain

import (
	crand "crypto/rand"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"runtime/debug"
	"runtime/pprof"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/console"
	"github.com/dop251/goja_nodejs/require"
)

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
var timelimit = flag.Int("timelimit", 0, "max time to run (in seconds)")

func readSource(filename string) ([]byte, error) {
	if filename == "" || filename == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(filename)
}

func load(vm *goja.Runtime, call goja.FunctionCall) goja.Value {
	p := call.Argument(0).String()
	b, err := readSource(p)
	if err != nil {
		panic(vm.ToValue(fmt.Sprintf("Could not read %s: %v", p, err)))
	}
	v, err := vm.RunScript(p, string(b))
	if err != nil {
		panic(err)
	}
	return v
}

func newRandSource() goja.RandSource {
	var seed int64
	if err := binary.Read(crand.Reader, binary.LittleEndian, &seed); err != nil {
		panic(fmt.Errorf("Could not read random bytes: %v", err))
	}
	return rand.New(rand.NewSource(seed)).Float64
}

func run() error {
	filename := flag.Arg(0)
	src, err := readSource(filename)
	if err != nil {
		return err
	}

	if filename == "" || filename == "-" {
		filename = "<stdin>"
	}

	vm := goja.New()
	vm.SetRandSource(newRandSource())

	new(require.Registry).Enable(vm)
	console.Enable(vm)

	vm.Set("load", func(call goja.FunctionCall) goja.Value {
		return load(vm, call)
	})

	vm.Set("readFile", func(name string) (string, error) {
		b, err := os.ReadFile(name)
		if err != nil {
			return "", err
		}
		return string(b), nil
	})

	if *timelimit > 0 {
		time.AfterFunc(time.Duration(*timelimit)*time.Second, func() {
			vm.Interrupt("timeout")
		})
	}

	//log.Println("Compiling...")
	prg, err := goja.Compile(filename, string(src), false)
	if err != nil {
		return err
	}
	//log.Println("Running...")
	_, err = vm.RunProgram(prg)
	//log.Println("Finished.")
	return err
}

func GojaMain() error {
	logFlags := log.Flags()
	log.SetFlags(0)
	defer func() {
		if x := recover(); x != nil {
			debug.Stack()
			panic(x)
		}
		log.SetFlags(logFlags)
	}()
	flag.Parse()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	if err := run(); err != nil {
		//fmt.Printf("err type: %T\n", err)
		switch err := err.(type) {
		case *goja.Exception:
			return errors.New(err.String())
		case *goja.InterruptedError:
			return errors.New(err.String())
		default:
			return err
		}
	}
	return nil
}

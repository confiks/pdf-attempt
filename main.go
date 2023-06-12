package main

import (
	"errors"
	"fmt"
	"github.com/dop251/goja"
	"io/ioutil"
	v8 "rogchap.com/v8go"
)

func main() {
	holderConfigJSONBts, err := ioutil.ReadFile("holder-config.json")
	if err != nil {
		fmt.Println("Could not read holder config JSON")
		return
	}

	vaccinationsJSONBts, err := ioutil.ReadFile("vaccination.json")
	if err != nil {
		fmt.Print("Could not read vaccination JSON")
		return
	}

	polyfillBts, err := ioutil.ReadFile("web-polyfill.js")
	if err != nil {
		fmt.Println("Could not read polyfill source file")
		return
	}

	pdfToolsBts, err := ioutil.ReadFile("pdf-tools.js")
	if err != nil {
		fmt.Println("Could not read source file")
		return
	}

	//MainV8(string(holderConfigJSONBts), string(vaccinationsJSONBts), string(polyfillBts), string(pdfToolsBts))
	MainGoja(string(holderConfigJSONBts), string(vaccinationsJSONBts), string(polyfillBts), string(pdfToolsBts))
}

func MainGoja(holderConfigJSON, vaccinationsJSON, polyfill, pdfTools string) {
	vm := goja.New()
	_, err := vm.RunScript("run.js", pdfTools)
	if err != nil {
		fmt.Printf("Could not run PDF tools script: %s\n", err)
		return
	}

	_, err = vm.RunScript("run.js", polyfill)
	if err != nil {
		fmt.Printf("Could not run PDF tools script: %s\n", err)
		return
	}

	vm.Set("holderConfigJSON", holderConfigJSON)
	vm.Set("vaccinationJSON", vaccinationsJSON)

	runScript := fmt.Sprintf(`
		var console = {
			error: function() {},
			log: function() {},
		}

		var setTimeoutCounter = 42;
		function setTimeout(f, n, ...args) {
			f(...args);
			return setTimeoutCounter++;
		}
		
		function clearTimeout() {
		}

		holderConfig = JSON.parse(holderConfigJSON);
		vaccination = JSON.parse(vaccinationJSON);
		var proofs = pdfTools.parseProofData(vaccination, holderConfig, 'nl');
		var options = {
			proofs,
			locale: 'nl',
			qrSizeInCm: 8,
			createdAt: new Date(),
			internationalProofScanned: false,
		};

		pdfTools.getDocument(options);
	`)

	pdfPromise, err := vm.RunScript("run.js", runScript)
	if err != nil {
		fmt.Printf("Could not complete running phase: %s\n", err)
		return
	}

	var result interface{}
	if p, ok := pdfPromise.Export().(*goja.Promise); ok {
		switch p.State() {
		case goja.PromiseStateRejected:
			panic(p.Result().String())
		case goja.PromiseStateFulfilled:
			result = p.Result().Export()
		default:
			panic("unexpected promise state pending")
		}
	}

	fmt.Println(result)
}

// -----------------

func explainV8Error(err error) {
	e := err.(*v8.JSError)
	fmt.Println(e.Message)
	fmt.Println(e.Location)
	fmt.Println(e.StackTrace)
}

func resolveV8Promise(ctx *v8.Context, val *v8.Value) (*v8.Value, error) {
	for {
		switch p, _ := val.AsPromise(); p.State() {
		case v8.Fulfilled:
			fmt.Println("fulfilled")
			return p.Result(), nil
		case v8.Rejected:
			fmt.Println("rejected")
			return nil, errors.New(p.Result().DetailString())
		case v8.Pending:
			// run VM to make progress on the promise
			ctx.PerformMicrotaskCheckpoint()
		default:
			return nil, fmt.Errorf("illegal v8.Promise state %d", p) // unreachable
		}
	}
}

func MainV8(holderConfigJSON, vaccinationsJSON, polyfill, pdfTools string) {
	iso := v8.NewIsolate()

	global := v8.NewObjectTemplate(iso)
	global.Set("holderConfigJSON", holderConfigJSON)
	global.Set("vaccinationJSON", vaccinationsJSON)

	ctx := v8.NewContext(iso, global)
	_, err := ctx.RunScript(polyfill, "run.js")
	if err != nil {
		fmt.Printf("Could not load web-polyfill.js library: %s\n", err)
		explainV8Error(err)

		return
	}

	_, err = ctx.RunScript(pdfTools, "run.js")
	if err != nil {
		fmt.Printf("Could not load pdf-tools.js library: %s\n", err)
		explainV8Error(err)

		return
	}

	runScript := fmt.Sprintf(`
		function setTimeout(f, n) {
			f();
		}
		
		function clearTimeout() {
		}

		holderConfig = JSON.parse(holderConfigJSON);
		vaccination = JSON.parse(vaccinationJSON);
		var proofs = pdfTools.parseProofData(vaccination, holderConfig, 'nl');
		var options = {
			proofs,
			locale: 'nl',
			qrSizeInCm: 8,
			createdAt: new Date(),
			internationalProofScanned: false,
		};

		pdfTools.getDocument(options);
	`)

	pdfPromise, err := ctx.RunScript(runScript, "run.js")
	if err != nil {
		fmt.Printf("Could not complete running phase: %s\n", err)
		explainV8Error(err)

		return
	}

	if !pdfPromise.IsPromise() {
		fmt.Println("meh")
		return
	}

	fmt.Println("Awaiting promise...")

	pdf, err := resolveV8Promise(ctx, pdfPromise)
	if err != nil {
		fmt.Printf("Could not resolve PDF promise %s\n", err)
		explainV8Error(err)

		return
	}

	fmt.Println(pdf)

	//v8.SetFlags("--max-heap-size=32 --initial-heap-size=2")
	//
	//ctx := v8.NewContext()
	//ctx.RunScript(string(scriptBts), "pdf-tools.js")

	//vm := goja.New()
	//v, err := vm.RunString(string(data))
	//if err != nil {
	//	panic(err)
	//}
	//
	//fmt.Println(v)
}

/*
	对glau功能的测试
*/
package glua

import (
	"fmt"
	"runtime"
	"testing"
	//rf "reflect"
)

func TestDofile(t *testing.T) {
	L := NewState()
	L.Openlibs()
	if !L.Dofile("testdofile.lua") {
		t.Error("Dofile error.")
	}
}

func TestDofilegc(t *testing.T) {
	runtime.GC()
}

func TestDostring(t *testing.T) {
	L := NewState()
	L.Openlibs()
	if !L.Dostring("print(2); print(math.pi)") {
		t.Error("Dofile error.")
	}
}

func TestDostringgc(t *testing.T) {
	runtime.GC()
}

func printf(b bool, i int, s string) (bool, int, string) {
	fmt.Println("this is a func in go test.")
	fmt.Println(b, i, s)
	return b, i, s
}

func getSlice() []int {
	return []int{1, 2, 3}
}

func printSlice(si []int) {
	fmt.Println(si)
}

var testfunc int

var tlib = Libfuncs{
	"gotest", // lib name
	map[string]interface{}{
		"printf":     printf,     // lua function name, go function
		"getSlice":   getSlice,   //
		"printSlice": printSlice, //
		"goprintln":  fmt.Println,
		"testfunc":   testfunc,  // error
	},
}

func TestRegLib(t *testing.T) {
	L := NewState()
	L.Openlibs()
	if ok, err := L.Register(&tlib); !ok {
		t.Fatal(err.Error())
	}
	if !L.Dofile("testregister.lua") {
		t.Error("Dofile error.")
	}
}

// 
func TestCall(t *testing.T) {
	L := NewState()
	L.Openlibs()
	if ok, err := L.Register(&tlib); !ok {
		t.Fatal(err.Error())
	}
	if out, ok := L.Call("print", 1, true, "print test"); !ok {
		t.Error("call print error.", out)
	}
	if out, ok := L.Call("gotest.goprintln", 1, true, "print test", getSlice()); !ok {
		t.Error("call fmt.Println error.", out)
	}
	if out, ok := L.Call("gotest.getSlice"); !ok {
		t.Error("call getSlice error.", out)
	} else {
		if slc, ok := out[0].([]int); !ok || slc[0] != 1 {
			t.Error("getSlice return error.", out[0], slc)
		}
	}
}

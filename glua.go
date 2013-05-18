/*
这是一个Go语言对lua5.1的简易binding.

实现的功能如下：
	1. 可在Go中创建多个虚拟机同时执行多个脚本
	2. 可以注册Go的库和函数到lua，结构方法需要转为函数才能注册。
	3. go的所有参数都可以传到lua中使用，但只有bool，int*，uint*，float*，string能转为
	   lua的boolean，number和string类型；其他类型一律转为userdata，只能传回go程序使用。
	   uint32和uint64类型使用可能会有精度损失，故推荐使用int代替。 
	4. 支持不定参数，支持多返回值，支持lua的不定参数


另外，编译需要修改头文件路径和链接的库：
	//#cgo CFLAGS: -I /usr/local/include/luajit-2.0 
	//#cgo LDFLAGS: -l luajit-5.1
*/
package glua

//#cgo CFLAGS: -I /usr/local/include/luajit-2.0 
//#cgo LDFLAGS: -l luajit-5.1
//#include <stdlib.h>
//#include <lua.h>
//#include <lauxlib.h>
//#include <lualib.h>
//#include "gluac.h"
import "C"
import (
	"errors"
	"fmt"
	"log"
	rf "reflect"
	"runtime"
	"unsafe"
)

//lua的状态结构
type State struct {
	s  *C.lua_State  // lua状态
	lf []interface{} // Gfucs列表
}

// 创建新的状态
func NewState() (L *State) {
	L = new(State)
	L.s = C.luaL_newstate()
	//L.lf = make([]interface{}, 5)
	// 保存State到lua栈中
	var li interface{} = L
	C.SetGoState(L.s, unsafe.Pointer(&li))
	// 生成metatable，用于回调Go函数
	C.InitMetaTable(L.s)
	// 注册回收
	runtime.SetFinalizer(L, (*State).free)
	log.Printf("Create %v.\n", *L)
	return
}

// 打开lua的所有标准库
func (L *State) Openlibs() {
	C.luaL_openlibs(L.s)
}

// 执行lua脚本
func (L *State) Dofile(fname string) (ok bool) {
	fn := C.CString(fname)
	defer C.free(unsafe.Pointer(fn))

	if C.luaL_loadfile(L.s, fn) == 0 {
		return C.lua_pcall(L.s, 0, C.LUA_MULTRET, 0) == 0
	}
	return false
}

// 执行lua脚本字符串
func (L *State) Dostring(str string) (ok bool) {
	cs := C.CString(str)
	defer C.free(unsafe.Pointer(cs))

	if C.luaL_loadstring(L.s, cs) == 0 {
		return C.lua_pcall(L.s, 0, C.LUA_MULTRET, 0) == 0
	}
	return false
}

// 注册Go库到lua
func (L *State) Register(lib *Libfuncs) (bool, error) {
	libn := C.CString(lib.Libname)
	defer C.free(unsafe.Pointer(libn))

	// 获取GlibTable
	fsize := len(lib.Funcs)
	if C.GetGlibTable(L.s, libn, C.int(fsize)) != 0 {
		return false, errors.New(fmt.Sprintf("Lib name(%s) is wrong.\n", lib.Libname))
	}

	// 设置函数
	for k, v := range lib.Funcs {
		// 检查函数列表
		if ok, err := checkFuncInOutArgs(v); !ok {
			log.Println(err)
			continue
		}
		// 保存到State
		L.lf = append(L.lf, v)
		idx := len(L.lf)
		kn := C.CString(k)
		// 设置index到GlibTtable
		C.SetGfunc(L.s, kn, C.int(idx-1))
		C.free(unsafe.Pointer(kn))
	}

	C.lua_settop(L.s, 0)

	return true, nil
}

// 调用lua中的函数，但是只能返回bool, float, string，以及其他go特殊类型，int型被转换为float返回。
func (L *State) Call(fname string, args ...interface{}) (out []interface{}, ok bool) {
	fn := C.CString(fname)
	defer C.free(unsafe.Pointer(fn))

	top := int(C.lua_gettop(L.s))

	if C.int(1) != C.FindFuncs(L.s, fn) {
		ok = false
		out = append(out, errors.New(fmt.Sprintf("not find the function(%s).\n", fname)))
		return 
	}

	num := len(args)
	for _, arg := range args {
		argt := rf.TypeOf(arg)
		argv := rf.ValueOf(arg)
		L.pushValueByType(argt.Kind(), &argv)
	}

	C.lua_call(L.s, C.int(num), C.LUA_MULTRET)

	for i := top; i < int(C.lua_gettop(L.s)); i++ {
		ret := L.getValueByLuaType(i)
		if ret.IsValid() {
		out = append(out, ret.Interface())
		} else {
		out = append(out, nil)
		}
	}
	C.lua_settop(L.s, C.int(top))
	ok = true
	return
}


// 释放lua_stat
func (L *State) free() {
	log.Printf("Free %v.\n", *L)
	C.lua_close(L.s)
}

//export gofuncCallback
func gofuncCallback(gs interface{}, idx int) (rn int) {
	L := gs.(*State)
	defer func() {
		if e := recover(); e != nil {
			L.pushString(fmt.Sprintf("%v\n", e))
			C.lua_error(L.s)
		}
	}()

	fv := rf.ValueOf(L.lf[idx])
	ft := rf.TypeOf(L.lf[idx])

	// 获取参数
	in := L.getFuncIn(ft)

	out := fv.Call(in)

	// 设置返回值
	L.setFuncOut(ft, out)

	return len(out)
}

func (L *State) getFuncIn(ft rf.Type) []rf.Value {
	var in []rf.Value
	var i int
	for i = 0; i < ft.NumIn()-1; i++ {
		in = append(in, *L.getValueByType(ft.In(i).Kind(), i))
	}

	switch {
	case ft.IsVariadic():
		ek := ft.In(i).Elem().Kind()
		for ; i < int(C.lua_gettop(L.s)); i++ {
			switch ek {
			case rf.Interface:
				in = append(in, *L.getValueByLuaType(i))
			default:
				in = append(in, *L.getValueByType(ek, i))
			}
		}
	case i < ft.NumIn():
		in = append(in, *L.getValueByType(ft.In(i).Kind(), i))
	}
	return in
}

func (L *State) getValueByLuaType(i int) (v *rf.Value) {
	switch C.lua_type(L.s, C.int(i+1)) {
	case C.LUA_TBOOLEAN:
		v = L.getValueByType(rf.Bool, i)
	case C.LUA_TNUMBER:
		v = L.getValueByType(rf.Float64, i)
	case C.LUA_TSTRING:
		v = L.getValueByType(rf.String, i)
	case C.LUA_TUSERDATA:
		v = L.getValueByType(rf.Interface, i)
	default:
		L.pushString("Wrong parameters.")
		C.lua_error(L.s)
		v = nil
	}
	return
}

func (L *State) getValueByType(vt rf.Kind, i int) *rf.Value {
	var v rf.Value
	switch vt {
	case rf.Bool:
		v = rf.ValueOf(L.getBool(i))
	case rf.Int:
		v = rf.ValueOf(int(L.getInt(i)))
	case rf.Int8:
		v = rf.ValueOf(int8(L.getInt(i)))
	case rf.Int16:
		v = rf.ValueOf(int16(L.getInt(i)))
	case rf.Int32:
		v = rf.ValueOf(int32(L.getInt(i)))
	case rf.Int64:
		v = rf.ValueOf(L.getInt(i))
	case rf.Uint:
		v = rf.ValueOf(uint(L.getInt(i)))
	case rf.Uint8:
		v = rf.ValueOf(uint8(L.getInt(i)))
	case rf.Uint16:
		v = rf.ValueOf(uint16(L.getInt(i)))
	case rf.Uint32:
		v = rf.ValueOf(uint32(L.getInt(i)))
	case rf.Uint64:
		v = rf.ValueOf(uint64(L.getInt(i)))
	case rf.String:
		v = rf.ValueOf(L.getString(i))
	case rf.Float32:
		v = rf.ValueOf(float32(L.getNumber(i)))
	case rf.Float64:
		v = rf.ValueOf(L.getNumber(i))
	//case rf.Uintptr, rf.Complex64, rf.Complex128, rf.Array, rf.Chan, rf.Func,
	//	rf.Interface, rf.Map, rf.Ptr, rf.Slice, rf.Struct, rf.UnsafePointer:
	default:
		v = rf.ValueOf(L.getInterface(i))
	}
	return &v
}

func (L *State) setFuncOut(ft rf.Type, out []rf.Value) {
	C.lua_settop(L.s, 0)
	for i, v := range out {
		L.pushValueByType(ft.Out(i).Kind(), &v)
	}
}

func (L *State) pushValueByType(vt rf.Kind, v *rf.Value) {
	switch vt {
	case rf.Bool:
		L.pushBool(v.Bool())
	case rf.Int, rf.Int8, rf.Int16, rf.Int32, rf.Int64:
		L.pushInt(v.Int())
	case rf.Uint, rf.Uint8, rf.Uint16, rf.Uint32, rf.Uint64:
		L.pushInt(int64(v.Uint()))
	case rf.String:
		L.pushString(v.String())
	case rf.Float32:
		L.pushNumber(float64(v.Float()))
	case rf.Float64:
		L.pushNumber(v.Float())
	//case rf.Uintptr, rf.Complex64, rf.Complex128, rf.Array, rf.Chan, rf.Func,
	//	rf.Interface, rf.Map, rf.Ptr, rf.Slice, rf.Struct, rf.UnsafePointer:
	default:
		L.pushInterface(v.Interface())
	}
}

func checkFuncInOutArgs(fn interface{}) (bool, error) {
	t := rf.TypeOf(fn)
	if t.Kind() != rf.Func {
		return false, errors.New(fmt.Sprintf("type(%v) is not a type(func).\n", t))
	}
	/*
		// in args check
		for i := 0; i < t.NumIn(); i++ {
			switch t.In(i).Kind() {
			case rf.Bool, rf.Int, rf.String, rf.Float32, rf.Float64,
				rf.Uintptr, rf.Complex64, rf.Complex128, rf.Array, rf.Chan, // rf.Func,
				rf.Interface, rf.Map, rf.Ptr, rf.Slice, rf.Struct, rf.UnsafePointer:
			// do nothing
			default:
				return false, errors.New(fmt.Sprintf(
					"in args type(%v) of func(%v) is not supported.\n", t.In(i).Kind(), t))
			}
		}
		// out args check
		for i := 0; i < t.NumOut(); i++ {
			switch t.Out(i).Kind() {
			case rf.Bool, rf.Int, rf.String, rf.Float32, rf.Float64,
				rf.Uintptr, rf.Complex64, rf.Complex128, rf.Array, rf.Chan, // rf.Func,
				rf.Interface, rf.Map, rf.Ptr, rf.Slice, rf.Struct, rf.UnsafePointer:
			// do nothing
			default:
				return false, errors.New(fmt.Sprintf(
					"out args type(%v) of func(%v) is not supported.\n", t.Out(i).Kind(), t))
			}
		}
	*/

	return true, nil
}

func (L *State) getBool(i int) (ret bool) {
	return (C.lua_toboolean(L.s, C.int(i+1)) == 1)
}

func (L *State) getInt(i int) (ret int64) {
	return int64(C.lua_tointeger(L.s, C.int(i+1)))
}

func (L *State) getString(i int) (ret string) {
	return C.GoString(C.lua_tolstring(L.s, C.int(i+1), nil))
}

func (L *State) getNumber(i int) (ret float64) {
	return float64(C.lua_tonumber(L.s, C.int(i+1)))
}

func (L *State) getInterface(i int) (ret interface{}) {
	C.GetInterface(L.s, unsafe.Pointer(&ret), C.int(i+1))
	return
}

func (L *State) pushBool(b bool) {
	if b {
		C.lua_pushboolean(L.s, 1)
	} else {
		C.lua_pushboolean(L.s, 0)
	}
}

func (L *State) pushInt(i int64) {
	C.lua_pushinteger(L.s, C.lua_Integer(i))
}

func (L *State) pushString(str string) {
	cs := C.CString(str)
	defer C.free(unsafe.Pointer(cs))
	C.lua_pushstring(L.s, cs)
}

func (L *State) pushNumber(f float64) {
	C.lua_pushnumber(L.s, C.lua_Number(f))
}

func (L *State) pushInterface(iface interface{}) {
	C.SetInterface(L.s, unsafe.Pointer(&iface))
}

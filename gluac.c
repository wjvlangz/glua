

#include <stdio.h>
#include <string.h>
#include "lua.h"
#include "lauxlib.h"
#include "lualib.h"
#include "_cgo_export.h"

static const char GoStateRegistryKey = 'k'; //golua registry key
static const char* GoMetaTableName   = "GLua.GoCallBack";

// 获取GoState 
static GoInterface getGoState(lua_State* L)
{
	//get gostate from registry entry
	lua_pushlightuserdata(L, (void*)&GoStateRegistryKey);
	lua_gettable(L, LUA_REGISTRYINDEX);
	GoInterface gip = *(GoInterface*)lua_touserdata(L, -1);
	lua_pop(L, 1);
	return gip;	
}

// 判断栈底是否Gfunc
static int checkGfunc(lua_State* L, int index)
{
	int* fidx = (int*)luaL_checkudata(L,index, GoMetaTableName);
	luaL_argcheck(L, fidx != NULL, index, "'GoFunction' expected");
	return *fidx;
}

// 注册到__call的回调函数
static int callback2gofunc(lua_State* L)
{
	int fidx = checkGfunc(L,1);
	GoInterface gi = getGoState(L);
	//remove the go function from the stack (to present same behavior as lua_CFunctions)
	lua_remove(L,1);
		
	return (int)gofuncCallback(gi, fidx);
}

// 将State注册为lua中的userdata
void SetGoState(lua_State* L, const void* gi)
{
    GoInterface* p = (GoInterface*)gi;
	lua_pushlightuserdata(L, (void*)&GoStateRegistryKey);
	GoInterface* gip = (GoInterface*)lua_newuserdata(L, sizeof(GoInterface));
	//copy interface value to userdata
	*gip = *p;
	//set into registry table
	lua_settable(L, LUA_REGISTRYINDEX);
}

// 初始化metatable
void InitMetaTable(lua_State* L)
{
	// 创建metatable，注册回调函数
	luaL_newmetatable(L, GoMetaTableName);
	//push function
	lua_pushcfunction(L, &callback2gofunc);
	//t[__call] = &callback_function
	lua_setfield(L, -2, "__call");
	
	lua_pop(L,1);
}

// 将Gfunc转换为userdata，并压入栈底
static void pushGoFunc(lua_State* L, int fid)
{
	int* fidptr = (int*)lua_newuserdata(L, sizeof(int));
	*fidptr = fid;
	// 设置metatable
	luaL_getmetatable(L, GoMetaTableName);
	lua_setmetatable(L, -2);
}


// 获取GlibTable，压入栈底
int GetGlibTable(lua_State* L, const char* libname, int lsize)
{
	/* check whether lib already exists */
	luaL_findtable(L, LUA_REGISTRYINDEX, "_LOADED", 1);
	lua_getfield(L, -1, libname);  /* get _LOADED[libname] */
	if (!lua_istable(L, -1)) 
	{  
		/* not found? */
		lua_pop(L, 1);  /* remove previous result */
		/* try global variable (and create one if it does not exist) */
		if (luaL_findtable(L, LUA_GLOBALSINDEX, libname, lsize) != NULL)
		{
			return -1;
		}
		lua_pushvalue(L, -1);
		lua_setfield(L, -3, libname);  /* _LOADED[libname] = new table */
	}
	lua_remove(L, -2);  /* remove _LOADED table */
	return 0;
}

// 设置Gfunc索引到栈底的Table
void SetGfunc(lua_State* L, const char* fname, int idx)
{
	// 栈底为GlibTable，此函数接GetGlibTable
	
	// 将Gfunc的索引压入栈
	pushGoFunc(L, idx);
	// 设置Gfunc到GlibTable
	lua_setfield(L, -2, fname);
}


// 获取interface数据
void GetInterface(lua_State* L, const void* iface, int i)
{
    GoInterface* p = (GoInterface*)iface;
	*p = *(GoInterface*)lua_touserdata(L, i);
}
// 设置interface数据
void SetInterface(lua_State* L, void* iface)
{
    GoInterface* p = (GoInterface*)iface;
	GoInterface* ud = (GoInterface*)lua_newuserdata(L, sizeof(GoInterface));
	//copy interface value to userdata
	*ud = *p;
}

int FindFuncs(lua_State* L, char* fname)
{
	char *e;
	int idx = LUA_GLOBALSINDEX;
	do {
		e = strchr(fname, '.');
		if (e != NULL)
		{
			*e = '\0';
		}
		lua_getfield(L, idx, fname);
		if (lua_isnil(L, -1))
		{
			lua_pop(L, 1);  /* remove this nil */
			return -1;
		}
		else
		{
			if (idx == LUA_GLOBALSINDEX)
			{
				idx = -1;
			}
			else
			{
				lua_remove(L, -2);
			}
			fname = e + 1;
		}
	} while (e != NULL);
	return 1;
}


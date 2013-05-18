/*

*/

#ifndef _GLUAC_H_
#define _GLUAC_H_


// 将State注册为lua中的userdata
extern void SetGoState(lua_State* L, const void* gi);
// 初始化metatable
extern void InitMetaTable(lua_State* L);

// 获取GlibTable，压入栈底
extern int GetGlibTable(lua_State* L, const char* libname, int lsize);
// 设置Gfunc索引到栈底的Table
extern void SetGfunc(lua_State* L, const char* fname, int idx);

// 获取interface数据
extern void GetInterface(lua_State* L, const void* iface, int i);
// 设置interface数据
extern void SetInterface(lua_State* L, void* iface);

extern int FindFuncs(lua_State* L, char* fname);

#endif 

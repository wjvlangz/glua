
print("call go function from lua.")

print(gotest.printf())

print(gotest.printf(true, 10, "from lua", gotest))
print(gotest.printf(true, true, true))
print(gotest.printf(10, 10, 10))
print(gotest.printf("from lua", "from lua", "from lua"))
print(gotest.printf(math, math, math))
print(gotest.printf(nil, nil, nil))

gotest.printSlice(gotest.getSlice())

gotest.goprintln()

gotest.goprintln(23.04, false, 1000, "456", "this is lua", gotest.getSlice())

gotest.goprintln()

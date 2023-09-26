/**
=========================================
This is function to analysis how parameter in golang
is being parsed to register.
From that we can determine how to extract offset from this information.

Sinh file ELF sử dụng lệnh:
```
GOOS=linux GOARCH=amd64 go tool compile -S -N -l main.go
```
=========================================
*/

package main

type testStruct struct {
	a int
	b int
	c string
}

func (t *testStruct) Hello() {
	t.a = 1
}

func (t *testStruct) Hi() {
	t.b = 1
}

func (t *testStruct) Ha(c *testStruct) (u int, v int) {
	t.b = 1
	c.b = 1

	u = 1
	v = 1
	return
}

type testInterface interface {
	Hello()
	Hi()
	Ha(*testStruct) (int, int)
}

type Fields map[string]interface{}

func main() {
	p2 := testStruct{
		a: 11,
		b: 22,
	}
	p3 := testStruct{
		a: 100,
		b: 200,
		c: "hello",
	}

	p2.Ha(&p3)
}

func TestField(c Fields) {
	c["a"] = 1
}

type Point struct {
	X int
	Y int
}

func (p *Point) PIncr(i testInterface, h interface{}, str string) int {
	i.Hello()

	if str == "123" {
		p.X += 10
		p.X += 10
		p.Y += h.(int)
	}

	return 1
}

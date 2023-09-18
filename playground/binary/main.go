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
}

func (t *testStruct) Hello() {
	t.a = 1
}

func (t *testStruct) Hi() {
	t.b = 1
}

type testInterface interface {
	Hello()
	Hi()
}

func main() {
	str := "12345"

	p := Point{2, 5}
	p2 := testStruct{}

	i := 1

	p.PIncr(&p2, i, str)
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

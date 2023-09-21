## Note 

- In new version of golang, the parameter is stored in register first, instead of stack.
- Trong nhưng version mới nhất của golang (>=1.17), các tham số hàm được lưu trữ trong thanh ghi trước, thay cho stack như các version trước.
- Đối với kiểu string.
  - Đầu tiên là một slot trỏ tới giá trị
  - Thứ 2 là 1 slot lưu trữ chiều dài.
- Đối với interface xác định (ví dụ ctx Context), cần 2 biến.
  - đầu tiên là interface, liên hệ tới các hàm của interface 
  - thứ 2 là nội dung của biến, liên hệ tới một con trỏ của struct
- Đối với kiểu interface {}, cần 2 biến.
  - đầu tiên là trỏ tới kiểu 
  - Thứ 2 là trỏ tới con trỏ của interface 
- Đối với kiểu map ?

## Áp dụng ví dụ với các thư viện đã được xâm nhập 

http/net/client 

```go
func (c *Client) Do(req *Request) (*Response, error) {
	return c.do(req)
}

--> request_pos bằng 2 do có 1 pointer receiver và 1 pointer request.
--> Cần xem cách lấy giá trị từ một pointer thông qua ví dụ này.
```

http/net/server

```go
func (f HandlerFunc) ServeHTTP(w ResponseWriter, r *Request) {
	f(w, r)
}

--> request_pos bằng 4 do có 1 pointer receiver, 1 interface => cần 2 slot, 1 con trỏ request
```

golang.org/grpc/

```go
func (cc *ClientConn) Invoke(ctx context.Context, method string, args, reply interface{}, opts ...CallOption)

--> request_pos bằng
- 4 và 5 tương ứng với nội dung và độ dài của method 
- 8 và 9 tương ứng với reply
	
do có 1 pointer receiver, 1 string => 2 slot, args, reply interface{} -> 2 slot. Còn lại bỏ qua.
```

database/sql/

```go
func (db *DB) queryDC(ctx, txctx context.Context, dc *driverConn, releaseConn func(error), query string, args []any) (*Rows, error) {
  queryerCtx, ok := dc.ci.(driver.QueryerContext)
  var queryer driver.Queryer
  ...
}

--> query position bằng 8 và 9, tương ứng với giá trị và slot.
do có 1 pointer receiver, 2 context -> 4 slot, 1 con trỏ, 1 hàm còn trỏ, 1 query string 2 slot. Phần còn lại bỏ qua.
```
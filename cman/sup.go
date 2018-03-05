/*
Copyright (c) 2018 Simon Schmidt

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

// UDP Connection Manager
package cman

import "net"
import "sync"
import "fmt"
import "time"

var ENotSupp = fmt.Errorf("Not Supported")
var EAddrInUse = fmt.Errorf("Address In Use")

var bufs = sync.Pool{ New:func()interface{} { return make([]byte,1500) } }

type UKey struct {
	IP [16]byte
	Port int
}

type Client struct {
	S *Server
	A *net.UDPAddr
	C chan []byte
	K UKey
}
func (c *Client) Read(b []byte) (int,error) {
	p := <- c.C
	i := len(p)
	j := len(b)
	copy(b,p)
	if cap(p)==1500 { bufs.Put(p[:1500]) }
	if i>j { i=j }
	return i,nil
}
func (c *Client) Write(b []byte) (int,error) {
	return c.S.U.WriteToUDP(b,c.A)
}
func (c *Client) Close() error {
	c.S.Lock(); defer c.S.Unlock()
	delete(c.S.C,c.K)
	close(c.C)
	return nil
}
func (c *Client) RemoteAddr() net.Addr {
	return c.A
}
func (c *Client) LocalAddr() net.Addr {
	return c.S.U.LocalAddr()
}

// Not supported
func (c *Client) SetDeadline(t time.Time) error { return ENotSupp }

// Not supported
func (c *Client) SetReadDeadline(t time.Time) error { return ENotSupp }

// Not supported
func (c *Client) SetWriteDeadline(t time.Time) error { return ENotSupp }

type Server struct {
	sync.Mutex
	U *net.UDPConn
	C map[UKey]*Client
}
func AsServer(u *net.UDPConn) *Server {
	return &Server{U:u,C:make(map[UKey]*Client)}
}
func (s *Server) Destroy() {
	for _,c := range s.C {
		close(c.C)
	}
}
func (s *Server) Accept() (*Client,error) {
	var key UKey
	bf := bufs.Get().([]byte)
	for {
		i,a,e := s.U.ReadFromUDP(bf)
		if e!=nil {
			bufs.Put(bf)
			return nil,e
		}
		copy(key.IP[:],a.IP.To16())
		key.Port = a.Port
		s.Lock()
		c,ok := s.C[key]
		if !ok {
			c = &Client{S:s,A:a,C:make(chan []byte,16),K:key}
			s.C[key] = c
			s.Unlock()
			c.C <- bf[:i]
			return c,nil
		}
		select {
		case c.C <- bf[:i]: bf = bufs.Get().([]byte)
		default: /* Otherwise drop packet. */
		}
		s.Unlock()
	}
}
func (s *Server) Create(a *net.UDPAddr) (*Client,error) {
	var key UKey
	copy(key.IP[:],a.IP.To16())
	key.Port = a.Port
	c := &Client{S:s,A:a,C:make(chan []byte,16),K:key}
	
	s.Lock()
	defer s.Unlock()
	_,ok := s.C[key]
	if ok { return nil,EAddrInUse }
	s.C[key] = c
	return c,nil
}

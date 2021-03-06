package sock

import (
	"github.com/golang/glog"
	"net"
)

type Client struct {
	address         string
	counter         uint32
	codecBuild      CodecBuild
	option          Option
	connectCallBack ConnectionCallBack
	closeCallBack   ConnectionCallBack
	readCallBack    ReadCallBack
}

func NewClient(address string) *Client {
	client := new(Client)
	client.address = address
	client.counter = 0
	return client
}

func (c *Client) OnConnect(callback ConnectionCallBack) {
	c.connectCallBack = callback
}

func (c *Client) onClose(callback ConnectionCallBack) {
	c.closeCallBack = callback
}
func (c *Client) OnRead(callback ReadCallBack) {
	c.readCallBack = callback
}

func (c *Client) Connect() error {
	cn, err := net.Dial("tcp", c.address)
	if err != nil {
		return err
	}
	tcpCon, _ := cn.(*net.TCPConn)
	c.counter = c.counter + 1
	index := c.counter
	con := NewConnection(tcpCon, index)
	con.setConnectCallBack(c.connectCallBack)
	con.setCloseCallBack(c.closeCallBack)
	con.setReadCallBack(c.readCallBack)
	con.establish()
	go con.readLoop()
	go con.writeLoop()
	glog.Infof("[Connect] connect a new connection %s\n", con.name)
	return nil
}

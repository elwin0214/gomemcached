package sock

import (
	"fmt"
	. "logger"
	"net"
	//"sync/atomic"
	"time"
	"util"
)

type Connection struct {
	//base
	id      uint32
	name    string
	tcpConn net.Conn
	//closeFlag int32
	closeFlag *util.AtomicInt
	codec     Codec

	//callback
	connectionCallBack ConnectionCallBack

	//read
	readCallBack ReadCallBack
	readTimeout  time.Duration

	//write
	lastWriteTime                   time.Time
	writeChan                       chan interface{}
	readWriteChannelTimeout         time.Duration
	readWriteChannelTimer           *time.Timer
	readWriteChannelTimeoutCallBack ConnectionCallBack
}

func NewConnection(tcpConn net.Conn, index uint32, codec Codec) *Connection {
	con := new(Connection)
	con.id = index
	con.name = fmt.Sprintf("%s-%d", tcpConn.RemoteAddr().String(), index)
	con.tcpConn = tcpConn
	//con.reader = bufio.NewReader(tcpConn)
	con.codec = codec
	con.writeChan = make(chan interface{}, 1024)
	con.closeFlag = util.NewAtomicInt(0)
	con.readWriteChannelTimeout = time.Millisecond * 6000
	con.readWriteChannelTimer = time.NewTimer(con.readWriteChannelTimeout)
	con.lastWriteTime = time.Now()
	return con
}

func (con *Connection) establish() {
	if nil != con.connectionCallBack {
		con.connectionCallBack(con)
	}
}
func (con *Connection) GetId() uint32 {
	return con.id
}

func (con *Connection) GetName() string {
	return con.name
}

func (con *Connection) setCoder(codec Codec) {
	con.codec = codec
}

func (con *Connection) setConnectionCallBack(callback ConnectionCallBack) {
	con.connectionCallBack = callback
}

func (con *Connection) setReadCallBack(callback ReadCallBack) {
	con.readCallBack = callback
}
func (con *Connection) SetReadWriteChannelTimeout(timeoutMs time.Duration) {
	con.readWriteChannelTimeout = time.Millisecond * timeoutMs
	con.readWriteChannelTimer.Reset(con.readWriteChannelTimeout)
}

func (con *Connection) SetReadWriteChannelTimeoutCallBack(callback ConnectionCallBack) {
	con.readWriteChannelTimeoutCallBack = callback
}

func (con *Connection) IsClosed() bool {
	return con.closeFlag.Get() == 1
}

func (con *Connection) Close() {
	if con.closeFlag.Cas(0, 1) {
		LOG.Info("[Close] goto close %s", con.GetName())
		con.tcpConn.Close()
		con.readWriteChannelTimer.Reset(0 * time.Millisecond)
		if nil != con.connectionCallBack {
			con.connectionCallBack(con)
			close(con.writeChan)
		}
	}
}

func (con *Connection) SetReadTimeout(timeoutMs time.Duration) {
	con.readTimeout = time.Millisecond * timeoutMs
}

func (con *Connection) readLoop() {
	if con.readTimeout > 0 {
		con.tcpConn.SetReadDeadline(time.Now().Add(con.readTimeout))
	}
	for {
		body, err := con.codec.Decode()
		if nil != err {
			LOG.Error("[readLoop] connection = %s, error = %s, goroute exit\n", con.GetName(), err.Error())
			con.Close()
			return
		}
		if con.readTimeout > 0 {
			con.tcpConn.SetReadDeadline(time.Now().Add(con.readTimeout))
		}
		if nil != con.readCallBack {
			con.readCallBack(con, &Message{Id: con.id, Body: body})
		} else {
			LOG.Debug("[readLoop] %s", "not exit read callback")
		}
	}
}

func (con *Connection) Send(body interface{}) {
	defer func() {
		if err := recover(); err != nil {
			LOG.Error("[Send] connecton = %d, msg = %s ", con.id, err)
		}
	}()
	con.writeChan <- body // if closed
}

func (con *Connection) Write(buffer []byte) {
	con.writeChan <- buffer
}

func (con *Connection) writeLoop() {
	for {
		select {
		//heart beat
		case <-con.readWriteChannelTimer.C:
			if con.IsClosed() {
				LOG.Info("[writeLoop] connection = %s, connection is closed, goroute exit.\n", con.GetName())
				return
			}
			distance := time.Now().Sub(con.lastWriteTime)
			LOG.Debug("[writeLoop] connection = %s, timeout, dis = %d", con.GetName(), distance)
			if distance < (con.readWriteChannelTimeout) {
				con.readWriteChannelTimer.Reset(con.readWriteChannelTimeout - distance)
			} else {
				if con.readWriteChannelTimeoutCallBack != nil {
					con.readWriteChannelTimeoutCallBack(con)
				}
				con.readWriteChannelTimer.Reset(con.readWriteChannelTimeout)
			}

		case body, ok := <-con.writeChan:
			if !ok {
				LOG.Error("[writeLoop] connection = %s, write channel is closed, goroute exit.", con.GetName())
				return
			}
			if con.IsClosed() {
				LOG.Error("[writeLoop] connection = %s, connection is closed, goroute exit.\n", con.GetName())
				return
			}
			err := con.codec.Encode(body)
			if nil != err {
				con.Close()
				LOG.Error("[writeLoop] connection = %s, error = %s, close conn, goroute exit.\n", con.GetName(), err.Error())
				return
			}
			con.lastWriteTime = time.Now()
			con.readWriteChannelTimer.Reset(con.readWriteChannelTimeout)
		}

	}
}

package serve

import (
	"log"
	"net/http"
)

type tmessage struct {
	content    []byte
	fromuser   []byte
	touser     []byte
	mtype      int // 1-系统消息 2-用户消息
	createtime string
}

// Hub 集线器维护活动连接集，并将消息广播到连接。
// Hub maintains the set of active connections and broadcasts messages to the connections.
type Hub struct {
	// Registered connections.
	//注册连接
	connections map[*connection]bool

	// Inbound messages from the connections.
	//连接中的绑定消息
	broadcast chan *tmessage

	// Register requests from the connections.
	//添加新连接
	register chan *connection

	// Unregister requests from connections.
	//删除连接
	unregister chan *connection
}

var h = Hub{
	//广播slice
	broadcast: make(chan *tmessage),
	//注册者slice
	register: make(chan *connection),
	//未注册者sclie
	unregister: make(chan *connection),
	//连接map
	connections: make(map[*connection]bool),
}

func init() {
	go h.run()
}

// ServeWs 处理客户端对websocket请求
// ServeWs handles websocket requests from the peer.
func ServeWs(w http.ResponseWriter, r *http.Request) {
	//设定环境变量
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	//初始化connection
	c := &connection{send: make(chan []byte, 256), ws: ws, auth: true}
	//加入注册通道，意思是只要连接的人都加入register通道
	h.register <- c
	go c.writePump() //服务器端发送消息给客户端
	c.readPump()     //服务器读取的所有客户端的发来的消息
}

func (h *Hub) run() {
	for {
		select {
		//注册者有数据，则插入连接map
		case c := <-h.register:
			h.connections[c] = true
		//非注册者有数据，则删除连接map
		case c := <-h.unregister:
			if _, ok := h.connections[c]; ok {
				delete(h.connections, c)
				close(c.send)
			}
		//广播有数据
		case m := <-h.broadcast:
			//递归所有广播连接
			for c := range h.connections {

				var sendMsg []byte
				if m.mtype == 1 { //系统消息
					sendMsg = []byte(" system: " + string(m.content))
				} else if m.mtype == 2 { //用户消息
					sendMsg = []byte(string(m.fromuser) + " say: " + string(m.content))
				} else {
					sendMsg = []byte(string(m.content))
				}

				var sendFlag = false
				if string(m.touser) == "all" {
					sendFlag = true
				} else {
					//  只有 发送者 和 接收者 能看到
					if string(c.username) == string(m.touser) || string(c.username) == string(m.fromuser) {
						sendFlag = true
					}
				}
				if sendFlag {
					select {
					//发送数据给连接
					case c.send <- sendMsg:
					//关闭连接
					default:
						close(c.send)
						delete(h.connections, c)
					}
				}
			}
		}
	}
}

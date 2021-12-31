package tcptls

import (
	"crypto/tls"
	"sync"
)


type ConnPool struct{
	pool map[string]*tls.Conn
	mutex sync.RWMutex
}

func newConnPool() ConnPool{
	return ConnPool{pool: make(map[string]*tls.Conn)}
}

func (p *ConnPool) GetConn(addr string) *tls.Conn{
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	if conn,ok:=p.pool[addr]; ok{
		return conn
	}
	return nil
}

func (p *ConnPool) AddConn(addr string,conn *tls.Conn){
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.pool[addr]=conn
	return
}

func (p *ConnPool) ConnExists(addr string) bool{
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	_,exists:=p.pool[addr]
	return exists
}

func (p *ConnPool) CloseConn(addr string){
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if conn,ok:=p.pool[addr]; ok{
		conn.Close()
		delete(p.pool,addr)
	}
	return
}

func (p *ConnPool) Close(){
	p.mutex.Lock()
	defer p.mutex.Unlock()
	for _,conn := range p.pool{
		conn.Close()
	}
	p.pool=nil //will be garbage collected
	return
}
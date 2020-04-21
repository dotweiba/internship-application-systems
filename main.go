package main

import (
    "flag"
    "fmt"
    "net"
    "math"
    "os"
    "os/signal"
    "syscall"
    "time"
    "encoding/binary"

    "golang.org/x/net/icmp"
    "golang.org/x/net/ipv4"
    //"golang.org/x/net/ipv6"
)

const (
    IDLen = 2
    SeqLen = 2
    TSLen = 8
)
var totalrtt[] time.Duration
var PkgRecv int = 0
var PkgSent int = 0
var seqnum int = 1
func Bytes2Int(b[]byte) int64 {
    return int64(binary.BigEndian.Uint64(b))
}

func Int2Bytes(i int64)[]byte {
    b:= make([]byte, 8)
    binary.BigEndian.PutUint64(b, uint64(i))
    return b
}

func Time2Bytes(t time.Time)[]byte {
    nsec:= uint64(t.UnixNano())
    b:= make([]byte, 8)
    binary.BigEndian.PutUint64(b, nsec)
    return b
}

func Bytes2Time(b[]byte) time.Time {
    nsec:= int64(binary.BigEndian.Uint64(b))
    return time.Unix(nsec / int64(math.Pow(10, 9)), nsec % int64(math.Pow(10, 9)))
}

func Goping(ip * net.IPAddr) {
    c, err:= net.ListenPacket("ip4:icmp", "0.0.0.0")
    if err != nil {
        fmt.Println("Unknown error. %v", err)
        os.Exit(2)
    }
    defer c.Close()
    id:= uint64(os.Getpid() & 0xffff)
    wm:= icmp.Message {
        Type: ipv4.ICMPTypeEcho,
        Code: 0,
        Body: & icmp.Echo {
            ID: os.Getpid() & 0xffff,
            Seq: seqnum,
            Data: Time2Bytes(time.Now()),
        },
    }
    seqnum++
    PkgSent++
    wb, err:= wm.Marshal(nil)
    if err != nil {
        fmt.Println("Unknown error.", err)
        os.Exit(3)
    }
    if _, err:= c.WriteTo(wb, ip);
    err != nil {
        fmt.Println("Unknown error.", err)
        os.Exit(4)
    }

    rb:= make([]byte, 1500)
    n, peer, err:= c.ReadFrom(rb)
    if err != nil {
        fmt.Println("Unknown error.", err)
        os.Exit(5)
    }
    rm, err:= icmp.ParseMessage(ipv4.ICMPTypeEcho.Protocol(), rb[: n])
    if err != nil {
        fmt.Println("Unknown error.", err)
        os.Exit(6)
    }
    switch rm.Type {
        case ipv4.ICMPTypeEchoReply:
            msgdata, _:= rm.Body.Marshal(ipv4.ICMPTypeEcho.Protocol())
            if len(msgdata) < IDLen + SeqLen + TSLen {
                fmt.Println("Insufficient Packet Recved, %d", len(msgdata))
                return
            }
            if id != 256 * uint64(msgdata[0]) + uint64(msgdata[1]) {
                fmt.Println("not the same id.")
                return
            }
            ts:= Bytes2Time(msgdata[IDLen + SeqLen: ])
            e:= time.Now().Sub(ts)
            fmt.Printf("got echo reply from %v, time=%v\n", peer, e)
            PkgRecv++
            totalrtt = append(totalrtt, e)
        default:
            fmt.Printf("got %v but not echo reply\n", rm)
    }
}

func isIPv4(ip net.IP) bool {
    return len(ip.To4()) == net.IPv4len
}

func isIPv6(ip net.IP) bool {
    return len(ip) == net.IPv6len
}

func gethostIP(host string)( * net.IPAddr, bool) {
    ip, err:= net.ResolveIPAddr("ip", host)
    if err != nil {
        return nil, false
    }
    if isIPv4(ip.IP) {
        return ip, true
    } else if isIPv6(ip.IP) {
        fmt.Println("ipv6 not supported.")
        return ip, false
    }
    return ip, false
}

func main() {
    SetupHandler()
    timeInterval:= flag.Float64("ti", 1.0, "Indicates the time interval of sending ECHO REQUEST")
    count:= flag.Int("count", math.MaxInt16, "Stops after <count> times.")
    flag.Parse()
    if len(flag.Args()) < 1 {
        fmt.Println("Provides Hostname or IP.")
        os.Exit(1)
    }

    ip, _:= gethostIP(flag.Arg(0))
    if ip == nil {
        fmt.Println("Cant resolve target.")
        os.Exit(11)
    }
    fmt.Printf("Ping %v(%v)\n", flag.Arg(0), ip)
    x:= 0
    for x < * count {
        go Goping(ip)
        time.Sleep(time.Duration(int( * timeInterval * 1000.0)) * time.Millisecond)
        x++
    }

    os.Exit(0)
}

func SetupHandler() {
    c:= make(chan os.Signal)
    signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
    go func(){
        <-c
        var sum time.Duration = 0
        for _, v:= range totalrtt {
            sum += v
        }
        fmt.Printf("\rTotal Loss: %0.2f %%\n", float64(100 * PkgRecv / PkgSent))
        fmt.Printf("Avg RTT = %v\n", sum / time.Duration(len(totalrtt)))
        os.Exit(0)
    }()

}

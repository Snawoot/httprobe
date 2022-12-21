package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/netip"
	"strings"

	"github.com/hashicorp/go-multierror"
)

type LoggingDialer struct {
	dialer *net.Dialer
}

func NewLoggingDialer() *LoggingDialer {
	return &LoggingDialer{
		dialer: &net.Dialer{
			Resolver: nil,
		},
	}
}

type dialResult struct {
	conn net.Conn
	err  error
}

func (d *LoggingDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	log.Printf("dialing %s://%s ...", network, addr)
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("unable to split host and port %q: %w", addr, err)
	}

	ipNetwork := "ip"
	switch {
	case strings.HasSuffix(network, "4"):
		ipNetwork = "ip4"
	case strings.HasSuffix(network, "6"):
		ipNetwork = "ip6"
	}

	resolvedAddrs, err := net.DefaultResolver.LookupNetIP(ctx, ipNetwork, host)
	if err != nil {
		return nil, fmt.Errorf("unable to resolve host %q: %w", host, err)
	}
	log.Printf("resolved (%q, %q) => %v", ipNetwork, host, resolvedAddrs)
	if len(resolvedAddrs) == 0 {
		return nil, fmt.Errorf("no addresses found for host %q", host)
	}

	resultCh := make(chan dialResult)
	childCtx, cl := context.WithCancel(ctx)
	for _, resolvedAddr := range resolvedAddrs {
		go func(address netip.Addr) {
			conn, err := d.dialer.DialContext(childCtx, network, net.JoinHostPort(address.String(), port))
			resultCh <- dialResult{
				conn: conn,
				err:  err,
			}
		}(resolvedAddr)
	}

	var (
		resultConn net.Conn
		resultErr  error
		success    bool
	)
	for _ = range resolvedAddrs {
		res := <-resultCh
		if res.err != nil {
			resultErr = multierror.Append(resultErr, res.err)
		} else {
			if success {
				res.conn.Close()
			} else {
				success = true
				resultConn = res.conn
				cl()
			}
		}
	}

	if success {
		log.Printf("DialContext(ctx, %q, %q) => (%s <=> %s)",
			network, addr, resultConn.LocalAddr().String(), resultConn.RemoteAddr().String())
		return resultConn, nil
	}
	cl()
	return nil, resultErr
}

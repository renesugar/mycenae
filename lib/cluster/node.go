package cluster

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/uol/gobol"
	pb "github.com/uol/mycenae/lib/proto"
	"github.com/uol/mycenae/lib/utils"
	"github.com/uol/mycenae/lib/wal"
	"go.uber.org/zap"
)

type node struct {
	address string
	port    int
	conf    *Config
	mtx     sync.RWMutex
	ptsCh   chan []*pb.Point
	metaCh  chan []*pb.Meta
	pts     []*pb.Point

	conn     *grpc.ClientConn
	client   pb.TimeseriesClient
	wLimiter *rate.Limiter
	rLimiter *rate.Limiter
	mLimiter *rate.Limiter
	wal      *wal.WAL
}

func newNode(address string, port int, conf *Config) (*node, gobol.Error) {

	//cred, err := newClientTLSFromFile(conf.Consul.CA, conf.Consul.Cert, conf.Consul.Key, "*")
	cred, err := credentials.NewClientTLSFromFile(conf.Consul.Cert, "localhost.consul.macs.intranet")
	if err != nil {
		return nil, errInit("newNode", err)
	}

	conn, err := grpc.Dial(fmt.Sprintf("%v:%d", address, port), grpc.WithTransportCredentials(cred))
	if err != nil {
		return nil, errInit("newNode", err)
	}

	logger.Debug(
		"new node",
		zap.String("package", "cluster"),
		zap.String("func", "newNode"),
		zap.String("addr", address),
		zap.Int("port", port),
	)

	ws := &wal.Settings{
		PathWAL:            filepath.Join(conf.LogPath, address),
		SyncInterval:       "1s",
		CheckPointInterval: "1s",
		CleanupInterval:    "1m",
	}

	w, err := wal.New(ws, logger)
	if err != nil {
		return nil, errInit("newNode", err)
	}

	if err := w.Open(); err != nil {
		return nil, errInit("newNode", err)
	}

	node := &node{
		address:  address,
		port:     port,
		conf:     conf,
		conn:     conn,
		ptsCh:    make(chan []*pb.Point, 5),
		metaCh:   make(chan []*pb.Meta, 5),
		wLimiter: rate.NewLimiter(rate.Limit(conf.GrpcMaxServerConn)*0.9, conf.GrpcBurstServerConn),
		rLimiter: rate.NewLimiter(rate.Limit(conf.GrpcMaxServerConn)*0.1, conf.GrpcBurstServerConn),
		mLimiter: rate.NewLimiter(rate.Limit(conf.GrpcMaxServerConn)*0.1, conf.GrpcBurstServerConn),
		client:   pb.NewTimeseriesClient(conn),
		wal:      w,
	}

	node.replay()

	return node, nil
}

func (n *node) writePoints(timeout time.Duration, pts []*pb.Point) error {

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := n.wLimiter.Wait(ctx); err != nil {
		return err
	}

	stream, err := n.client.Write(ctx)
	if err != nil {
		return err
	}

	for _, p := range pts {
		var attempts int
		var err error
		for {
			attempts++
			err = stream.Send(p)
			if err == io.EOF {
				return nil
			}
			if err == nil {
				break
			}

			logger.Error(
				"retry write stream",
				zap.String("package", "cluster"),
				zap.String("func", "write"),
				zap.Int("attempt", attempts),
				zap.Error(err),
			)
			if attempts >= 5 {
				break
			}
		}

		if err != nil && err != io.EOF {
			return err
		}
	}

	_, err = stream.CloseAndRecv()
	if err != nil && err != io.EOF {
		return err
	}

	return nil
}

func (n *node) write(pts []*pb.Point) error {

	err := n.writePoints(n.conf.gRPCtimeout, pts)
	if err != nil {
		logger.Error(
			"sending points to replay log",
			zap.String("package", "cluster"),
			zap.String("func", "write"),
			zap.String("error", err.Error()),
			zap.Error(err),
		)
		n.send2wal(pts)
		return err
	}

	return nil
}

func (n *node) read(ksid, tsid string, start, end int64) ([]*pb.Point, gobol.Error) {

	ctx, cancel := context.WithTimeout(context.Background(), n.conf.gRPCtimeout)
	defer cancel()

	if err := n.rLimiter.Wait(ctx); err != nil {
		return []*pb.Point{}, errRequest("node/read", http.StatusInternalServerError, err)
	}

	stream, err := n.client.Read(ctx, &pb.Query{Ksid: ksid, Tsid: tsid, Start: start, End: end})
	if err != nil {
		return []*pb.Point{}, errRequest("node/read", http.StatusInternalServerError, err)
	}

	var pts []*pb.Point
	for {
		p, err := stream.Recv()
		if err == io.EOF {
			// read done.
			return pts, nil
		}
		if err != nil {
			return pts, errRequest("node/write", http.StatusInternalServerError, err)
		}

		pts = append(pts, p)
	}

}

func (n *node) meta(metas []*pb.Meta) (<-chan *pb.MetaFound, error) {

	ctx, cancel := context.WithTimeout(context.Background(), n.conf.gRPCtimeout)

	if err := n.mLimiter.Wait(ctx); err != nil {
		logger.Error(
			"meta request limit",
			zap.String("package", "cluster"),
			zap.String("func", "node/meta"),
			zap.Error(err),
		)
		return nil, err
	}

	stream, err := n.client.GetMeta(ctx)
	if err != nil {
		logger.Error(
			"meta gRPC problem",
			zap.String("package", "cluster"),
			zap.String("func", "node/meta"),
			zap.Error(err),
		)
		return nil, err
	}

	go func() {
		for _, m := range metas {
			err := stream.Send(m)
			if err == io.EOF {
				return
			}
			if err != nil {
				logger.Error(
					"meta gRPC send problem",
					zap.String("package", "cluster"),
					zap.String("func", "node/meta"),
					zap.Error(err),
				)
			}
		}

		err := stream.CloseSend()
		if err != nil {
			logger.Error(
				"meta gRPC CloseSend problem",
				zap.String("package", "cluster"),
				zap.String("func", "node/meta"),
				zap.Error(err),
			)
		}
	}()

	c := make(chan *pb.MetaFound, len(metas))
	go func() {
		defer close(c)
		defer cancel()
		for _ = range metas {
			mf, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				logger.Error(
					"meta gRPC receive problem",
					zap.String("package", "cluster"),
					zap.String("func", "node/meta"),
					zap.Error(err),
				)
				continue
			}

			c <- mf
		}
	}()

	return c, nil

}

func (n *node) close() {
	err := n.conn.Close()
	if err != nil {
		logger.Error(
			"closing connection",
			zap.String("package", "cluster"),
			zap.String("func", "node/close"),
			zap.Error(err),
		)
	}
}

func (n *node) send2wal(pts []*pb.Point) {
	valuesMap := make(map[string][]wal.Value)

	for _, p := range pts {
		ksts := string(utils.KSTS(p.GetKsid(), p.GetTsid()))
		valuesMap[ksts] = append(valuesMap[ksts], wal.NewFloatValue(p.GetDate(), float64(p.GetValue())))
	}

	segID, err := n.wal.WriteMulti(valuesMap)
	if err != nil {
		logger.Error(
			err.Error(),
			zap.String("package", "cluster"),
			zap.String("struct", "node"),
			zap.String("func", "send2wal"),
			zap.Error(err),
			zap.Int64("segID", segID),
		)
	}
}

func (n *node) replay() {

	log := logger.With(
		zap.String("package", "cluster"),
		zap.String("struct", "node"),
		zap.String("func", "replay"),
	)

	go func() {
		ticker := time.NewTicker(time.Minute)

		for {

			lrt := time.Now()

			<-ticker.C

			lwt := n.wal.LastWriteTime()

			if lwt.After(lrt) {
				names, err := wal.SegmentFileNames(n.wal.Path())
				if err != nil {
					log.Error(
						"error getting list of files",
						zap.Error(err),
					)

				}

				count := len(names)
				for i, segmentFile := range names {
					pts, err := n.wal.Replay(segmentFile)
					if err != nil {
						log.Error(
							err.Error(),
							zap.Error(err),
						)
					}

					timeout := time.Duration(len(pts)) * time.Second
					err = n.writePoints(timeout, pts)
					if err != nil {
						logger.Error(
							"replaying points",
							zap.String("package", "cluster"),
							zap.String("struct", "node"),
							zap.String("func", "replay"),
							zap.String("error", err.Error()),
							zap.Error(err),
						)
						continue
					}

					logger.Debug(
						"points replayed",
						zap.String("package", "cluster"),
						zap.String("struct", "node"),
						zap.String("func", "replay"),
						zap.Int("count", len(pts)),
						zap.String("logfile", segmentFile),
					)

					if i+1 == count {
						continue
					}

					err = n.wal.Remove([]string{segmentFile})
					if err != nil {
						logger.Error(
							"removing replay log",
							zap.String("package", "cluster"),
							zap.String("struct", "node"),
							zap.String("func", "replay"),
							zap.String("error", err.Error()),
							zap.Error(err),
						)
						continue
					}
				}
			}
		}

	}()
}

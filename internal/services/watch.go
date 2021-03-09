package services

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/authzed/spicedb/internal/datastore"
	api "github.com/authzed/spicedb/pkg/REDACTEDapi/api"
	"github.com/authzed/spicedb/pkg/zookie"
)

type watchServer struct {
	api.UnimplementedWatchServiceServer

	ds datastore.TupleDatastore
}

// NewWatchServer creates an instance of the watch server.
func NewWatchServer(ds datastore.TupleDatastore) api.WatchServiceServer {
	s := &watchServer{ds: ds}
	return s
}

func (ws *watchServer) Watch(req *api.WatchRequest, stream api.WatchService_WatchServer) error {

	if len(req.Namespaces) == 0 {
		status.Error(codes.InvalidArgument, "watch request must contain one or more namespaces")
	}
	namespaceMap := make(map[string]struct{})
	for _, ns := range req.Namespaces {
		namespaceMap[ns] = struct{}{}
	}
	filter := namespaceFilter{namespaces: namespaceMap}

	var afterRevision uint64 = 0
	if req.StartRevision != nil && req.StartRevision.Token != "" {
		decodedRevision, err := zookie.Decode(req.StartRevision)
		if err != nil {
			status.Errorf(codes.InvalidArgument, "failed to decode start revision: %s", err)
		}

		afterRevision = decodedRevision.GetV1().Revision
	} else {
		var err error
		afterRevision, err = ws.ds.Revision()
		if err != nil {
			status.Errorf(codes.Unavailable, "failed to start watch: %s", err)
		}
	}

	updates, errchan := ws.ds.Watch(stream.Context(), afterRevision)
	for {
		select {
		case update, ok := <-updates:
			if ok {
				filtered := filter.filterUpdates(update.Changes)
				if len(filtered) > 0 {
					stream.Send(&api.WatchResponse{
						Updates:     update.Changes,
						EndRevision: zookie.NewFromRevision(update.Revision),
					})
				}
			}
		case err := <-errchan:
			switch err {
			case datastore.ErrWatchCanceled:
				return status.Errorf(codes.Canceled, "watch canceled by user: %s", err)
			case datastore.ErrWatchDisconnected:
				return status.Errorf(codes.ResourceExhausted, "watch disconnected: %s", err)
			default:
				return status.Errorf(codes.Internal, "watch error: %s", err)
			}
		}
	}
}

type namespaceFilter struct {
	namespaces map[string]struct{}
}

func (nf namespaceFilter) filterUpdates(candidates []*api.RelationTupleUpdate) []*api.RelationTupleUpdate {
	var filtered []*api.RelationTupleUpdate

	for _, update := range candidates {
		if _, ok := nf.namespaces[update.Tuple.ObjectAndRelation.Namespace]; ok {
			filtered = append(filtered, update)
		}
	}

	return filtered
}

package channel

import (
	"atlas-world/kafka/producer"
	"atlas-world/rest"
	"errors"
	"github.com/Chronicle20/atlas-model/model"
	"github.com/Chronicle20/atlas-rest/server"
	tenant "github.com/Chronicle20/atlas-tenant"
	"github.com/gorilla/mux"
	"github.com/manyminds/api2go/jsonapi"
	"github.com/sirupsen/logrus"
	"net/http"
	"strconv"
)

const (
	GetChannelServers       = "get_channel_servers"
	RegisterChannelServer   = "register_channel_server"
	UnregisterChannelServer = "unregister_channel_server"
	getChannel              = "get_channel"
)

func InitResource(si jsonapi.ServerInformation) server.RouteInitializer {
	return func(router *mux.Router, l logrus.FieldLogger) {
		registerGet := rest.RegisterHandler(l)(si)
		registerDelete := rest.RegisterHandler(l)(si)

		r := router.PathPrefix("/worlds/{worldId}/channels").Subrouter()
		r.HandleFunc("", registerGet(GetChannelServers, handleGetChannelServers)).Methods(http.MethodGet)
		r.HandleFunc("", rest.RegisterInputHandler[RestModel](l)(si)(RegisterChannelServer, handleRegisterChannelServer)).Methods(http.MethodPost)
		r.HandleFunc("/{channelId}", registerDelete(UnregisterChannelServer, handleUnregisterChannelServer)).Methods(http.MethodDelete)
		r.HandleFunc("/{channelId}", registerGet(getChannel, handleGetChannel)).Methods(http.MethodGet)
	}
}

func handleGetChannelServers(d *rest.HandlerDependency, c *rest.HandlerContext) http.HandlerFunc {
	return rest.ParseWorldId(d.Logger(), func(worldId byte) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			cs, err := GetByWorld(d.Logger())(d.Context())(worldId)
			if err != nil {
				d.Logger().WithError(err).Errorf("Unable to get all channel servers.")
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			res, err := model.SliceMap(Transform)(model.FixedProvider(cs))(model.ParallelMap())()
			if err != nil {
				d.Logger().WithError(err).Errorf("Creating REST model.")
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			server.Marshal[[]RestModel](d.Logger())(w)(c.ServerInformation())(res)
		}
	})
}

func handleRegisterChannelServer(d *rest.HandlerDependency, c *rest.HandlerContext, input RestModel) http.HandlerFunc {
	return rest.ParseWorldId(d.Logger(), func(worldId byte) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			id, err := strconv.Atoi(input.GetID())
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			t := tenant.MustFromContext(d.Context())
			_ = producer.ProviderImpl(d.Logger())(d.Context())(EnvEventTopicChannelStatus)(emitChannelServerStarted(t, worldId, byte(id), input.IpAddress, input.Port))
			w.WriteHeader(http.StatusAccepted)
		}
	})
}

func handleUnregisterChannelServer(d *rest.HandlerDependency, c *rest.HandlerContext) http.HandlerFunc {
	return rest.ParseWorldId(d.Logger(), func(worldId byte) http.HandlerFunc {
		return rest.ParseChannelId(d.Logger(), func(channelId byte) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				ch, err := GetById(d.Logger())(d.Context())(worldId, channelId)
				if err != nil {
					d.Logger().WithError(err).Errorf("Attempting to shutdown a world [%d] channel [%d] that does not exist.", worldId, channelId)
					w.WriteHeader(http.StatusNotFound)
					return
				}

				t := tenant.MustFromContext(d.Context())
				_ = producer.ProviderImpl(d.Logger())(d.Context())(EnvEventTopicChannelStatus)(emitChannelServerShutdown(t, worldId, channelId, ch.IpAddress(), ch.Port()))
				w.WriteHeader(http.StatusAccepted)
			}
		})
	})
}

func handleGetChannel(d *rest.HandlerDependency, c *rest.HandlerContext) http.HandlerFunc {
	return rest.ParseWorldId(d.Logger(), func(worldId byte) http.HandlerFunc {
		return rest.ParseChannelId(d.Logger(), func(channelId byte) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				ch, err := GetById(d.Logger())(d.Context())(worldId, channelId)
				if err != nil {
					if errors.Is(err, errChannelNotFound) {
						w.WriteHeader(http.StatusNotFound)
						return
					}

					d.Logger().WithError(err).Errorf("Unable to get channel.")
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				res, err := model.Map(Transform)(model.FixedProvider(ch))()
				if err != nil {
					d.Logger().WithError(err).Errorf("Creating REST model.")
					w.WriteHeader(http.StatusInternalServerError)
					return
				}

				server.Marshal[RestModel](d.Logger())(w)(c.ServerInformation())(res)
			}
		})
	})
}

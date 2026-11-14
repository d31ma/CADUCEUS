package media

import (
	"errors"
	"net"

	"github.com/pion/ice/v4"
	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/cc"
	"github.com/pion/interceptor/pkg/gcc"
	"github.com/pion/sdp/v3"
	"github.com/pion/webrtc/v4"
)

type PeerFactory struct {
	settings     webrtc.SettingEngine
	startBitrate int
}

func NewPeerFactory(options PeerOptions, connection net.PacketConn) PeerFactory {
	settings := webrtc.SettingEngine{}
	settings.SetICEUDPMux(webrtc.NewICEUDPMux(nil, connection))
	settings.SetNetworkTypes([]webrtc.NetworkType{webrtc.NetworkTypeUDP4, webrtc.NetworkTypeUDP6})
	settings.SetICETimeouts(options.DisconnectedTimeout, options.FailedTimeout, options.KeepAliveInterval)
	settings.SetIncludeLoopbackCandidate(options.IncludeLoopback)
	settings.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
	settings.SetNAT1To1IPs(options.PublicAddresses, webrtc.ICECandidateTypeHost)
	return PeerFactory{settings: settings, startBitrate: options.StartBitrate}
}

func (factory PeerFactory) New() (PeerParts, error) {
	engine, err := factory.mediaEngine()
	if err != nil {
		return PeerParts{}, err
	}
	bitrate := NewBitrateHolder(factory.startBitrate)
	registry, err := factory.interceptors(engine, bitrate)
	if err != nil {
		return PeerParts{}, err
	}
	api := webrtc.NewAPI(webrtc.WithMediaEngine(engine), webrtc.WithSettingEngine(factory.settings), webrtc.WithInterceptorRegistry(registry))
	peer, err := api.NewPeerConnection(webrtc.Configuration{})
	return PeerParts{Peer: peer, Api: api, Bitrate: bitrate}, err
}

func (PeerFactory) mediaEngine() (*webrtc.MediaEngine, error) {
	engine := &webrtc.MediaEngine{}
	return engine, errors.Join(
		engine.RegisterCodec(Codecs{}.Opus(), webrtc.RTPCodecTypeAudio),
		engine.RegisterCodec(Codecs{}.Vp8(), webrtc.RTPCodecTypeVideo),
		engine.RegisterHeaderExtension(webrtc.RTPHeaderExtensionCapability{URI: sdp.AudioLevelURI}, webrtc.RTPCodecTypeAudio),
		engine.RegisterHeaderExtension(webrtc.RTPHeaderExtensionCapability{URI: ExtensionIds{}.FrameMarkingUri()}, webrtc.RTPCodecTypeVideo),
		webrtc.ConfigureSimulcastExtensionHeaders(engine),
	)
}

func (factory PeerFactory) interceptors(engine *webrtc.MediaEngine, bitrate *BitrateHolder) (*interceptor.Registry, error) {
	registry := &interceptor.Registry{}
	congestion, congestionErr := cc.NewInterceptor(factory.estimator)
	err := errors.Join(
		congestionErr,
		webrtc.RegisterDefaultInterceptors(engine, registry),
		webrtc.ConfigureTWCCHeaderExtensionSender(engine, registry),
	)
	if err != nil {
		return nil, err
	}
	congestion.OnNewPeerConnection(bitrate.Set)
	registry.Add(congestion)
	return registry, nil
}

func (factory PeerFactory) estimator() (cc.BandwidthEstimator, error) {
	return gcc.NewSendSideBWE(gcc.SendSideBWEInitialBitrate(factory.startBitrate), gcc.SendSideBWEPacer(gcc.NewNoOpPacer()))
}

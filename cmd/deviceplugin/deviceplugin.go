// Copyright (C) 2023 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path"
	"time"

	"github.com/csl-svc/excat/pkg/rdtcat"
	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

const (
	loglevel           = zerolog.DebugLevel
	timeout            = 5
	cacheLevel2        = 2
	cacheLevel3        = 3
	rootResourceName   = "intel.com/"
	rdtAnnotation      = "io.kubernetes.cri.rdt-class"
	rdtCrirmAnnotation = "rdtclass.cri-resource-manager.intel.com/pod"
	resctrlPath        = "/sys/fs/resctrl"
	resourceBaseName   = "excat"
)

// Buffer keeps the rdt cat class name and the according device struct
type Buffer struct {
	device pluginapi.Device
	name   string
}

// ExcatDevicePlugin implements the Kubernetes device plugin API
type ExcatDevicePlugin struct {
	buffers      []*Buffer
	resourceName string
	socket       string
	server       *grpc.Server
	cacheLevel   int
}

// patchStringValue keeps payload to patch node labels
type patchStringValue struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value"`
}

// NewExcatDevicePlugin returns an initialized ExcatDevicePlugin
func NewExcatDevicePlugin(
	resourceName string, cacheLevel int, socket string, buffers []*Buffer,
) *ExcatDevicePlugin {
	return &ExcatDevicePlugin{
		resourceName: resourceName,
		cacheLevel:   cacheLevel,
		socket:       socket,
		server:       nil,
		buffers:      buffers,
	}
}

func main() {
	initLogger()

	// get initial list of devices
	log.Debug().Msg("Get initial buffer list")

	// read all buffers from /sys/fs/resctrl
	resctrl := rdtcat.Resctrl{}
	resctrl.ExcatBuffers = &resctrl
	allRdtBuffers := rdtcat.Buffers{
		Resctrl: resctrl,
	}

	if err := allRdtBuffers.GetAllBuffers(); err != nil {
		log.Fatal().Err(err).Msgf("error when reading buffers from %v", resctrlPath)
	}

	if err := allRdtBuffers.CreateLabels(); err != nil {
		log.Fatal().Err(err).Msg("error when creating node labels")
	}

	rmAllLabels()

	var dpl2, dpl3 *ExcatDevicePlugin

	cacheLevel := [2]int{2, 3}
	plugins := [2]*ExcatDevicePlugin{dpl2, dpl3}

	// extract buffers referring to cache level and create device plugin for each supported cache level
	for i := 0; i < 2; i++ {
		rdtBuffers := allRdtBuffers.ExtractBuffers(cacheLevel[i])

		if rdtBuffers.ResctrlGroups != nil {
			var countBuffers int

			// add node label for respective cache level
			var label string

			switch cacheLevel[i] {
			case cacheLevel2:
				label = allRdtBuffers.DpL2Label
			case cacheLevel3:
				label = allRdtBuffers.DpL3Label
			}

			if err := addNodeLabel(cacheLevel[i], label); err != nil {
				log.Fatal().Err(err).Msg("error when patching node labels")
			}

			// create all buffers as used by the device plugin
			resourceName := fmt.Sprintf("%s-l%v", resourceBaseName, cacheLevel[i])
			socketName := fmt.Sprintf("%sintel-excat-l%v", pluginapi.DevicePluginPath, cacheLevel[i])

			var buffers []*Buffer

			for _, buf := range rdtBuffers.ResctrlGroups {
				if buf.Name != rdtcat.DefaultClass {
					dev := pluginapi.Device{
						ID:     rootResourceName + resourceName + "-" + buf.Name,
						Health: pluginapi.Healthy,
					}

					buffers = append(buffers, &Buffer{
						device: dev,
						name:   buf.Name,
					})

					countBuffers++
				}
			}

			// create device plugin
			plugins[i] = NewExcatDevicePlugin(resourceName, cacheLevel[i], socketName, buffers)
			if err := plugins[i].Start(); err != nil {
				log.Fatal().Err(err).Msgf("error when creating device plugin for ExCAT with cache level %v", cacheLevel[i])
			}

			log.Info().Msgf("successfully started device plugin for %v cache level %v buffers", countBuffers, cacheLevel[i])
		} else {
			log.Info().Msgf("no cache level %v buffers configured", cacheLevel[i])
		}
	}

	// stay in main
	for {
	}
}

// initLogger initializes the logger for the device plugin
func initLogger() {
	zerolog.SetGlobalLevel(loglevel)

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// intitialize rdtcat logger
	rdtcat.InitLogger(loglevel)

	log.Info().Msgf("Logging level = %v.", loglevel)
	log.Debug().Msg("Logger initialized.")
}

// initialize initializes the ExcatDevicePlugin
func (b *ExcatDevicePlugin) initialize() {
	log.Debug().Msgf("Initialize ExcatDevicePlugin %v.", b.resourceName)
	b.server = grpc.NewServer([]grpc.ServerOption{}...)
}

// Start starts the gRPC server and registers the device plugin
func (b *ExcatDevicePlugin) Start() error {
	b.initialize()

	// Create socket and start gRPC server
	if err := b.Serve(); err != nil {
		return fmt.Errorf("could not start the gRPC server: %w", err)
	}

	// Register device plugin with specified resource
	if err := b.RegisterDevicePluginResource(); err != nil {
		return fmt.Errorf("could not register device plugin for resource %v with Kubelet: %w",
			b.resourceName, err)
	}

	return nil
}

// Serve creates the socket and starts the gRPC server
func (b *ExcatDevicePlugin) Serve() error {
	// create socket
	log.Debug().Msgf("Create socket %v.", b.socket)

	finfo, err := os.Stat(b.socket)
	if err == nil && finfo != nil {
		err = os.Remove(b.socket)
		if err != nil {
			return fmt.Errorf("error when removing %v: %w", b.socket, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("error when os.Stat %w", err)
	}

	var socket net.Listener

	socket, err = net.Listen("unix", b.socket)
	if err != nil {
		return fmt.Errorf("error when opening socket %v: %w", b.socket, err)
	}

	// start gRPC server
	log.Debug().Msgf("Start gRPC server with socket %v.", socket)

	// StartGrpcServer starts the gRPC server
	StartGrpcServer := func(b *ExcatDevicePlugin, socket net.Listener) {
		log.Debug().Msg("Starting the gRPC server...")

		err = b.server.Serve(socket)
		if err != nil {
			log.Fatal().Err(err).Msg("Couldn't start gRPC server.")
		}
	}

	go StartGrpcServer(b, socket)

	// register device plugin with the Kubelet
	pluginapi.RegisterDevicePluginServer(b.server, b)

	log.Debug().Msgf("Wait for gRPC server to be available. Timeout = %v seconds.", timeout)

	var conn *grpc.ClientConn

	conn, err = b.dial(b.socket, timeout*time.Second)
	if err != nil {
		return err
	}

	err = conn.Close()
	if err != nil {
		return err
	}

	return nil
}

// dial opens a connection to a socket and waits based on a blocking
// connection until the connection is successful
func (b *ExcatDevicePlugin) dial(socket string, timeout time.Duration) (*grpc.ClientConn, error) {
	log.Debug().Msgf("Connect to gRPC server at socket %v.", socket)
	conn, err := grpc.Dial(socket, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(timeout),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout) //nolint:wrapcheck // ok
		}))
	if err != nil { //nolint:wsl  // err check cuddled
		return nil, fmt.Errorf("failed to connect to gRPC server: %w", err)
	}

	return conn, nil
}

// RegisterDevicePluginResource registers a device plugin's resource with the Kubelet
func (b *ExcatDevicePlugin) RegisterDevicePluginResource() error {
	// Open connection to Kubelet
	log.Debug().Msgf("Connect to KubeletSocket %v.", pluginapi.KubeletSocket)

	conn, err := b.dial(pluginapi.KubeletSocket, timeout*time.Second)
	if err != nil {
		return err
	}

	defer conn.Close()

	// get new client
	log.Debug().Msg("Get new client.")

	client := pluginapi.NewRegistrationClient(conn)

	// register device plugin for specific resource
	resourceDNS := rootResourceName + b.resourceName
	req := &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     path.Base(b.socket),
		ResourceName: resourceDNS,
	}

	log.Debug().Msgf("Register device plugin with resource %v.", resourceDNS)

	if _, err := client.Register(context.Background(), req); err != nil {
		return fmt.Errorf("error when trying to register the device plugin with resource %v: %w",
			b.resourceName, err)
	}

	return nil
}

// GetDevicePluginOptions returns options to be communicated with Device
// Manager
func (b *ExcatDevicePlugin) GetDevicePluginOptions(
	context.Context, *pluginapi.Empty,
) (*pluginapi.DevicePluginOptions, error) {
	var devicePluginOptions pluginapi.DevicePluginOptions

	return &devicePluginOptions, nil
}

// ListAndWatch returns a stream of List of Devices
// Whenever a Device state change or a Device disappears, ListAndWatch
// returns the new list
func (b *ExcatDevicePlugin) ListAndWatch(
	e *pluginapi.Empty, listAndWatchServer pluginapi.DevicePlugin_ListAndWatchServer,
) error {
	if err := b.sendBuffers(listAndWatchServer); err != nil {
		return err
	}

	if err := b.watchBuffers(listAndWatchServer); err != nil {
		return err
	}

	return nil
}

// watchBuffers creates a fsnotify based watcher and adds all configured
// buffers directories to the watcher. Based on the type of change event,
// actions like re-reading the buffer configs are initiated.
func (b *ExcatDevicePlugin) watchBuffers(
	listAndWatchServer pluginapi.DevicePlugin_ListAndWatchServer,
) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("error when starting new fsnotify watcher: %w", err)
	}
	defer watcher.Close()

	done := make(chan struct{})

	// create parallel thread for checking watcher events.
	// due to how the RDT kernel driver works, events are only received for tasks
	// files that a PID is added to. For tasks files that a PID is removed from
	// (e.g. due to a deleted pod/container) no event is triggered.
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				log.Debug().Msgf("Change event: %v", event)

				if path.Base(event.Name) == "tasks" {
					if err := checkTasks(event.Name); err != nil {
						log.Error().Msgf("%v", err)
					}

					continue
				}

				log.Info().Msgf("Change event in buffers: %v", event)

				// for all changes (tasks files excluded) re-read buffer configs
				// and advertise them
				if err := b.updateBuffers(); err != nil {
					log.Error().Msgf("%v", err)
				}

				if err := b.sendBuffers(listAndWatchServer); err != nil {
					log.Error().Msgf("%v", err)
				}

			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}

				log.Error().Msgf("Error when watching %v: %v", resctrlPath, err)
			}
		}
	}()

	// add all buffer directories to the watcher
	for _, buffer := range b.buffers {
		bufferPath := resctrlPath + "/" + buffer.name
		if err := watcher.Add(bufferPath); err != nil {
			return fmt.Errorf("error when adding buffer directory to watcher: %w", err)
		}

		log.Debug().Msgf("Added %v to watcher.", bufferPath)
	}

	<-done

	return nil
}

// checkTasks reads in the provided tasks file and checks for contained PIDs.
func checkTasks(file string) error {
	time.Sleep(1 * time.Second) // sleep to get final container PID

	resctrl := rdtcat.Resctrl{}
	resctrl.ExcatBuffers = &resctrl

	pids, err := resctrl.GetBufferPids(file)
	if err != nil {
		return fmt.Errorf("error when checking PIDs in %v: %w", file, err)
	}

	log.Debug().Msgf("PID %v has been added to %v.", pids, file)

	// check if buffer is free again
	// TODO: for detection of removed PIDs replace this check with a
	// method that works, e.g. polling after being triggered by
	// write to other tasks file or freed up ER
	if pids == nil {
		bufferName := path.Base(path.Dir(file))
		log.Debug().Msgf("Buffer %v available again.", bufferName)
	}

	return nil
}

// sendBuffers loops through the buffers and sends the current list to the
// ListAndWatch server.
func (b *ExcatDevicePlugin) sendBuffers(listAndWatchServer pluginapi.DevicePlugin_ListAndWatchServer) error {
	var devs []*pluginapi.Device

	for _, buffer := range b.buffers {
		devs = append(devs, &buffer.device)
		log.Debug().Msgf("Appended buffer %v to devices", buffer.name)
	}

	log.Debug().Msg("Sending updated devices to ListAndWatchServer")

	if err := listAndWatchServer.Send(&pluginapi.ListAndWatchResponse{Devices: devs}); err != nil {
		return fmt.Errorf("error when sending updated buffers: %w", err)
	}

	return nil
}

// updateBuffers reads in the current configuration in /sys/fs/rescrtl and
// extracts the relevant buffers for the given cache level.
func (b *ExcatDevicePlugin) updateBuffers() error {
	// read all buffers from /sys/fs/resctrl
	resctrl := rdtcat.Resctrl{}
	resctrl.ExcatBuffers = &resctrl
	allRdtBuffers := rdtcat.Buffers{
		Resctrl: resctrl,
	}

	if err := allRdtBuffers.GetAllBuffers(); err != nil {
		return fmt.Errorf("error when reading buffers from %v: %w", resctrlPath, err)
	}

	// recreate labels
	if err := allRdtBuffers.CreateLabels(); err != nil {
		return fmt.Errorf("error when creating node labels: %w", err)
	}

	// rm possible old label
	rmNodeLabel(b.cacheLevel, "")

	// extract buffers for current cache level
	rdtBuffers := allRdtBuffers.ExtractBuffers(b.cacheLevel)

	// update buffers and labels
	if rdtBuffers.ResctrlGroups != nil {
		var (
			countBuffers int
			label        string
		)

		switch b.cacheLevel {
		case cacheLevel2:
			label = allRdtBuffers.DpL2Label
		case cacheLevel3:
			label = allRdtBuffers.DpL3Label
		}

		if err := addNodeLabel(b.cacheLevel, label); err != nil {
			return fmt.Errorf("error when patching node label: %w", err)
		}

		var buffers []*Buffer

		for _, buf := range rdtBuffers.ResctrlGroups {
			if buf.Name != rdtcat.DefaultClass {
				dev := pluginapi.Device{
					ID:     rootResourceName + b.resourceName + "-" + buf.Name,
					Health: pluginapi.Healthy,
				}

				buffers = append(buffers, &Buffer{
					device: dev,
					name:   buf.Name,
				})

				countBuffers++
			}
		}

		b.buffers = buffers

		log.Info().Msgf("Detected %v buffers in %v for cache level %v.", countBuffers, resctrlPath, b.cacheLevel)
	} else {
		log.Info().Msgf("No more buffers for cache level %v configured in %v.", b.cacheLevel, resctrlPath)
	}

	return nil
}

// Allocate is called during container creation so that the Device
// Plugin can run device specific operations and instruct Kubelet
// of the steps to make the Device available in the container
func (b *ExcatDevicePlugin) Allocate(
	ctx context.Context, allocateReqs *pluginapi.AllocateRequest,
) (*pluginapi.AllocateResponse, error) {
	allocateResp := pluginapi.AllocateResponse{}

	for _, allocateReq := range allocateReqs.ContainerRequests {
		if len(allocateReq.DevicesIDs) > 1 {
			return nil, fmt.Errorf("only one ExCAT buffer allowed per container: found request for %v",
				allocateReq.DevicesIDs)
		}

		name, err := b.id2name(allocateReq.DevicesIDs[0])
		if err != nil {
			return nil, err
		}

		cAllocateResp := pluginapi.ContainerAllocateResponse{}
		cAllocateResp.Annotations = make(map[string]string, 2) //nolint:gomnd // 2 annotations

		// add annotation for containerd and cri-o
		cAllocateResp.Annotations[rdtAnnotation] = name
		log.Debug().Msgf("Added the following annotation: \"%v\" = \"%v\"",
			rdtAnnotation, cAllocateResp.Annotations[rdtAnnotation])

		// add annotation for CRI-RM
		cAllocateResp.Annotations[rdtCrirmAnnotation] = name

		allocateResp.ContainerResponses = append(allocateResp.ContainerResponses, &cAllocateResp)
	}

	return &allocateResp, nil
}

// id2name extracts the buffer's name for a given device plugin ID
func (b *ExcatDevicePlugin) id2name(id string) (string, error) {
	for _, buffer := range b.buffers {
		if buffer.device.ID == id {
			return buffer.name, nil
		}
	}

	return "", fmt.Errorf("requested buffer with device ID = %v does not exist", id)
}

// GetPreferredAllocation returns a preferred set of devices to allocate
// from a list of available ones. The resulting preferred allocation is not
// guaranteed to be the allocation ultimately performed by the
// devicemanager. It is only designed to help the devicemanager make a more
// informed allocation decision when possible.
func (b *ExcatDevicePlugin) GetPreferredAllocation(
	context.Context, *pluginapi.PreferredAllocationRequest,
) (*pluginapi.PreferredAllocationResponse, error) {
	var par pluginapi.PreferredAllocationResponse

	return &par, nil
}

// PreStartContainer is called, if indicated by Device Plugin during registeration phase,
// before each container start. Device plugin can run device specific operations
// such as resetting the device before making devices available to the container.
func (b *ExcatDevicePlugin) PreStartContainer(
	context.Context, *pluginapi.PreStartContainerRequest,
) (*pluginapi.PreStartContainerResponse, error) {
	var pcr pluginapi.PreStartContainerResponse

	return &pcr, nil
}

package modem

import (
	"context"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/godbus/dbus/v5"
)

const ModemSimInterface = ModemManagerInterface + ".Sim"

type SIMs struct {
	modem *Modem
}

func (m *Modem) SIMs() *SIMs {
	return &SIMs{modem: m}
}

type SIM struct {
	dbusObject         dbus.BusObject
	Path               dbus.ObjectPath
	Active             bool
	Identifier         string
	Eid                string
	Imsi               string
	OperatorIdentifier string
	OperatorName       string
	GID1               string
}

func (s *SIMs) Primary(ctx context.Context) (*SIM, error) {
	if s.modem.Sim == nil {
		return nil, errors.New("primary SIM not available")
	}
	return s.Get(ctx, s.modem.Sim.Path)
}

func (sims *SIMs) Reference(path dbus.ObjectPath) (*SIM, error) {
	if path == "" || path == "/" {
		return nil, errors.New("SIM path is required")
	}
	sim := &SIM{Path: path}
	if sims.modem.dbusConn != nil {
		sim.dbusObject = sims.modem.dbusConn.Object(ModemManagerInterface, path)
	}
	return sim, nil
}

func (sims *SIMs) Get(ctx context.Context, path dbus.ObjectPath) (*SIM, error) {
	if path == "" || path == "/" {
		return nil, errors.New("SIM path is required")
	}
	var variant dbus.Variant
	var err error
	sim, err := sims.Reference(path)
	if err != nil {
		return nil, err
	}
	var dbusObject dbus.BusObject
	if sim.dbusObject != nil {
		dbusObject = sim.dbusObject
	} else {
		dbusObject, err = systemBusObject(path)
		if err != nil {
			return nil, err
		}
		sim.dbusObject = dbusObject
	}

	variant, err = dbusProperty(ctx, dbusObject, ModemSimInterface, "Active")
	if err != nil {
		return nil, err
	}
	sim.Active = boolFromVariant(variant)

	variant, err = dbusProperty(ctx, dbusObject, ModemSimInterface, "SimIdentifier")
	if err != nil {
		return nil, err
	}
	sim.Identifier = stringFromVariant(variant)

	variant, err = dbusProperty(ctx, dbusObject, ModemSimInterface, "Eid")
	if err != nil {
		return nil, err
	}
	sim.Eid = stringFromVariant(variant)

	variant, err = dbusProperty(ctx, dbusObject, ModemSimInterface, "Imsi")
	if err != nil {
		return nil, err
	}
	sim.Imsi = stringFromVariant(variant)

	variant, err = dbusProperty(ctx, dbusObject, ModemSimInterface, "OperatorIdentifier")
	if err != nil {
		return nil, err
	}
	sim.OperatorIdentifier = stringFromVariant(variant)

	variant, err = dbusProperty(ctx, dbusObject, ModemSimInterface, "OperatorName")
	if err != nil {
		return nil, err
	}
	sim.OperatorName = stringFromVariant(variant)

	if variant, err = dbusProperty(ctx, dbusObject, ModemSimInterface, "Gid1"); err == nil {
		sim.GID1 = strings.ToUpper(hex.EncodeToString(bytesFromVariant(variant)))
	}
	return sim, nil
}

func (s *SIM) SendPin(ctx context.Context, pin string) error {
	if s == nil || !validSIMObjectPath(s.Path) {
		return errors.New("SIM path is required")
	}
	dbusObject := s.dbusObject
	if dbusObject == nil {
		var err error
		dbusObject, err = systemBusObject(s.Path)
		if err != nil {
			return err
		}
	}
	return dbusObject.CallWithContext(ctx, ModemSimInterface+".SendPin", 0, pin).Err
}

# sinty-aegis

Sinty Aegis is the public alert and emergency call service of SintyOS. It
follows the official alert feeds and earthquake catalogues for the place the
computer is in, tells the desktop when something there needs attention, and
knows which numbers reach the emergency services in that country.

## Why it exists

A phone gets a national alert from the mobile network. A computer has no such
channel, so it stays silent while the phone on the desk is ringing. Aegis
gives SintyOS the same awareness from the sources the authorities publish on
the internet, for every country that publishes them.

## What it implements

- **Official alerts** in the Common Alerting Protocol. The list of feeds comes
  from the register of alerting authorities kept by the World Meteorological
  Organization, so the sources for a country are the ones its own authorities
  registered. Aegis reads the feeds of the country the computer is in, keeps
  the messages that are real, public and still valid, and shows the ones whose
  area contains the position of the computer. Feeds that name an area instead
  of drawing it are matched against the places the user chose.
- **Earthquakes** from the EMSC and USGS catalogues, worldwide. An earthquake
  is reported when it is strong enough to be felt where the computer is, and
  the two catalogues are merged so one event is one alert.
- **The hold.** An alert stays until the user dismisses it, and it cannot be
  dismissed in the first five minutes.
- **Emergency numbers** for 240 countries and territories, and a voice call
  through ModemManager on a computer that has a phone line. Only numbers that
  are emergency numbers in the current country can be called.
- **Emergency contacts and medical details** the desktop can show on the lock
  screen.

The service is `dev.sinty.Aegis1` on the session bus, at `/dev/sinty/Aegis1`:
`Alerts`, `Dismiss`, `Test`, `Config`, `SetConfig`, `Areas`, `Sos`, `Call` and
the `Changed` signal. Alerts, configuration and the SOS summary are JSON.

## Where the place comes from

The country and the position are the ones in the configuration. Until the user
sets them, Aegis takes both from the time zone of the system, which is enough
to pick the right feeds and the right emergency numbers.

## Limits

Alerts arrive as fast as their source publishes them, which is minutes, not
seconds: an earthquake report is information after the event, not an early
warning. A national alert sent only over the mobile network reaches Aegis only
if the authority also publishes it online.

## Build

    make build            # ./sinty-aegisd
    make release          # static stripped binary for the image
    make test

Requires Go >= 1.25 and Linux. The only Go dependency is
`github.com/godbus/dbus/v5`.

## Data

The emergency numbers in `internal/sos/numbers.go` are generated from the
short number metadata of libphonenumber, under the Apache License 2.0.

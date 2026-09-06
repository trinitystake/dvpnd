// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"

	sdkmath "cosmossdk.io/math"
	v1base "github.com/sentinel-official/sentinelhub/v12/types/v1"
	"github.com/showwin/speedtest-go/speedtest"
)

func FindInternetSpeed() (*v1base.Bandwidth, error) {
	_, err := speedtest.FetchUserInfo()
	if err != nil {
		return nil, err
	}

	servers, err := speedtest.FetchServers()
	if err != nil {
		return nil, err
	}

	servers, err = servers.FindServer(nil)
	if err != nil {
		return nil, err
	}

	var (
		upload   = sdkmath.LegacyZeroDec()
		download = sdkmath.LegacyZeroDec()
	)

	for _, s := range servers {
		s.Context.Reset()
		if err = s.PingTest(nil); err != nil {
			continue
		}

		if err = s.DownloadTest(); err != nil {
			continue
		}
		s.Context.Wait()

		if err = s.UploadTest(); err != nil {
			continue
		}
		s.Context.Wait()

		upload = sdkmath.LegacyMustNewDecFromStr(fmt.Sprintf("%f", s.ULSpeed))
		download = sdkmath.LegacyMustNewDecFromStr(fmt.Sprintf("%f", s.DLSpeed))

		if upload.IsPositive() && download.IsPositive() {
			break
		}
	}

	return &v1base.Bandwidth{
		Upload:   upload.Mul(sdkmath.LegacyNewDec(1e6)).QuoInt64(8).TruncateInt(),
		Download: download.Mul(sdkmath.LegacyNewDec(1e6)).QuoInt64(8).TruncateInt(),
	}, nil
}

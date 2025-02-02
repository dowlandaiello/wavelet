// Copyright (c) 2019 Perlin
//
// Permission is hereby granted, free of charge, to any person obtaining a copy of
// this software and associated documentation files (the "Software"), to deal in
// the Software without restriction, including without limitation the rights to
// use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
// the Software, and to permit persons to whom the Software is furnished to do so,
// subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
// FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
// COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
// IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
// CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package wavelet

import (
	"github.com/perlin-network/noise/skademlia"
	"github.com/perlin-network/wavelet/sys"
	"sync"
)

type vote struct {
	voter     *skademlia.ID
	preferred *Round
}

func CollectVotes(accounts *Accounts, snowball *Snowball, voteChan <-chan vote, wg *sync.WaitGroup) {
	votes := make([]vote, 0, sys.SnowballK)
	voters := make(map[AccountID]struct{}, sys.SnowballK)

	for vote := range voteChan {
		if _, recorded := voters[vote.voter.PublicKey()]; recorded {
			continue // To make sure the sampling process is fair, only allow one vote per peer.
		}

		voters[vote.voter.PublicKey()] = struct{}{}
		votes = append(votes, vote)

		if len(votes) == cap(votes) {
			snapshot := accounts.Snapshot()

			counts := make(map[RoundID]float64, len(votes))
			stakes := make(map[RoundID]float64, len(votes))

			for _, vote := range votes {
				if vote.preferred == nil {
					vote.preferred = ZeroRoundPtr
				}

				counts[vote.preferred.ID] += 1.0

				stake, _ := ReadAccountStake(snapshot, vote.voter.PublicKey())

				if stake < sys.MinimumStake {
					stake = sys.MinimumStake
				}

				stakes[vote.preferred.ID] += float64(stake)
			}

			maxCount := float64(0)
			maxStake := float64(0)

			for _, vote := range votes {
				if vote.preferred == nil {
					vote.preferred = ZeroRoundPtr
				}

				if maxCount < counts[vote.preferred.ID] {
					maxCount = counts[vote.preferred.ID]
				}

				if maxStake < stakes[vote.preferred.ID] {
					maxStake = stakes[vote.voter.PublicKey()]
				}
			}

			for _, vote := range votes {
				if vote.preferred == nil {
					vote.preferred = ZeroRoundPtr
				}

				counts[vote.preferred.ID] = (counts[vote.preferred.ID] / maxCount) * (stakes[vote.preferred.ID] / maxStake)
			}

			var majority *Round

			for _, vote := range votes {
				if vote.preferred == nil {
					vote.preferred = ZeroRoundPtr
				}

				if counts[vote.preferred.ID] >= sys.SnowballAlpha {
					majority = vote.preferred
					break
				}
			}

			snowball.Tick(majority)

			voters = make(map[AccountID]struct{}, sys.SnowballK)
			votes = votes[:0]
		}
	}

	if wg != nil {
		wg.Done()
	}
}

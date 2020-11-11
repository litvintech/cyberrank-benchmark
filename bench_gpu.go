// +build cuda

package main

import "C"
import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"strconv"
	"time"

	"github.com/cybercongress/cyberd/merkle"
	"github.com/spf13/cobra"
)

/*
#cgo CFLAGS: -I/usr/lib/
#cgo LDFLAGS: -L/usr/local/cuda/lib64 -lcbdrank -lcudart
#include "cbdrank.h"
*/
import "C"

func RunBenchGPUCmd() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "run-bench-gpu <stakesCount> <cidsCount> <dampingFactor> <tolerance>",
		Short: "Run rank calculation on GPU",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {

			stakesCount, _ := strconv.ParseInt(args[0], 10, 64)
			cidsCount, _ := strconv.ParseInt(args[1], 10, 64)
			dampingFactor, _ := strconv.ParseFloat(args[2], 64)
			tolerance, _ := strconv.ParseFloat(args[3], 64)

			fmt.Println("Agents: ", stakesCount)
			fmt.Println("CIDs: ", cidsCount)
			fmt.Println("Damping: ", dampingFactor)
			fmt.Println("Tolerance: ", tolerance)

			start := time.Now()

			outLinks := make(map[CidNumber]CidLinks)
			inLinks := make(map[CidNumber]CidLinks)
			stakes := make([]uint64, stakesCount)

			readStakesFromBytesFile(&stakes, "./stakes.data")
			readLinksFromBytesFile(&outLinks, "./outLinks.data")
			readLinksFromBytesFile(&inLinks, "./inLinks.data")
			fmt.Println("Graph open data: ", "time", time.Since(start))

			linksCount := uint64(0)
			rank := make([]float64, cidsCount)
			rankUint := make([]uint64, cidsCount)
			entropy := make([]float64, cidsCount)
			entropyUint := make([]uint64, cidsCount)
			inLinksCount := make([]uint32, cidsCount)
			outLinksCount := make([]uint32, cidsCount)
			inLinksOuts := make([]uint64, 0)
			inLinksUsers := make([]uint64, 0)
			outLinksUsers := make([]uint64, 0)

			start = time.Now()
			for i := int64(0); i < cidsCount; i++ {

				if inLinks, sortedCids, ok := GetSortedInLinks(inLinks, CidNumber(i)); ok {
					for _, cid := range sortedCids {
						inLinksCount[i] += uint32(len(inLinks[cid]))
						for acc := range inLinks[cid] {
							inLinksOuts = append(inLinksOuts, uint64(cid))
							inLinksUsers = append(inLinksUsers, uint64(acc))
						}
					}
					linksCount += uint64(inLinksCount[i])
				}

				if outLinks, ok := outLinks[CidNumber(i)]; ok {
					for _, accs := range outLinks {
						outLinksCount[i] += uint32(len(accs))
						for acc := range accs {
							outLinksUsers = append(outLinksUsers, uint64(acc))
						}
					}
				}
			}
			fmt.Println("Links amount", linksCount)
			fmt.Println("Stakes amount", len(stakes))
			fmt.Println("Data preparation", "time", time.Since(start))

			outLinks = nil
			inLinks = nil

			cStakes := (*C.ulong)(&stakes[0])

			cStakesSize := C.ulong(len(stakes))
			cCidsSize := C.ulong(len(inLinksCount))
			cLinksSize := C.ulong(len(inLinksOuts))

			cInLinksCount := (*C.uint)(&inLinksCount[0])
			cOutLinksCount := (*C.uint)(&outLinksCount[0])

			cInLinksOuts := (*C.ulong)(&inLinksOuts[0])
			cInLinksUsers := (*C.ulong)(&inLinksUsers[0])
			cOutLinksUsers := (*C.ulong)(&outLinksUsers[0])

			cDampingFactor := C.double(dampingFactor)
			cTolerance := C.double(tolerance)

			start = time.Now()
			cRank := (*C.double)(&rank[0])
			cEntropy := (*C.double)(&entropy[0])
			C.calculate_rank(
				cStakes, cStakesSize, cCidsSize, cLinksSize,
				cInLinksCount, cOutLinksCount,
				cInLinksOuts, cInLinksUsers, cOutLinksUsers,
				cRank, cDampingFactor, cTolerance, cEntropy,
			)
			fmt.Println("Rank calculation", "time", time.Since(start))

			fmt.Println("---------------------------------")

			start = time.Now()
			r := float64(0)
			for _, r64 := range rank {
				r += r64
			}
			fmt.Println("Ranks reduction: ", "time", time.Since(start))
			fmt.Printf("RanksSum: %f\n", r)

			fmt.Println("---------------------------------")

			start = time.Now()
			for i, f64 := range rank {
				rankUint[i] = uint64(f64*1e10)
			}
			fmt.Println("Rank converting to uint: ", "time", time.Since(start))
			fmt.Println("Ranks []float64: ", rank)
			fmt.Println("Ranks []uint64: ", rankUint)

			fmt.Println("---------------------------------")

			start = time.Now()
			rankTree := merkle.NewTree(sha256.New(), true)
			for _, r64 := range rankUint {
				rankBytes := make([]byte, 8)
				binary.LittleEndian.PutUint64(rankBytes, r64)
				rankTree.Push(rankBytes)
			}
			rhash := rankTree.RootHash()
			fmt.Println("Rank constructing merkle tree: ", "time", time.Since(start))
			fmt.Printf("Rank merkle root hash: %x\n", rhash)

			fmt.Println("---------------------------------")

			start = time.Now()
			e := float64(0)
			for _, e64 := range entropy {
				e += e64
			}
			fmt.Println("Entropy reduction: ", "time", time.Since(start))
			fmt.Printf("Entropy: %f\n", e)

			fmt.Println("---------------------------------")

			start = time.Now()
			for i, e64 := range entropy {
				entropyUint[i] = uint64(e64*1e10)
			}
			fmt.Println("Entropy converting to uint: ", "time", time.Since(start))
			fmt.Println("Entropy []float64: ", entropy)
			fmt.Println("Entropy []uint64: ", entropyUint)

			fmt.Println("---------------------------------")

			start = time.Now()
			entropyTree := merkle.NewTree(sha256.New(), true)
			for _, e64 := range entropyUint {
				entropyBytes := make([]byte, 8)
				binary.LittleEndian.PutUint64(entropyBytes, e64)
				entropyTree.Push(entropyBytes)
			}
			ehash := entropyTree.RootHash()
			fmt.Println("Entropy constructing merkle tree: ", "time", time.Since(start))
			fmt.Printf("Entropy merkle root hash: %x\n", ehash)

			fmt.Println("---------------------------------")

			//start = time.Now()
			//for i, l64 := range light {
			//	lightUint[i] = uint64(l64*1e10)
			//}
			//fmt.Println("Light converting to uint: ", "time", time.Since(start))
			//fmt.Println("Light []float64: ", light)
			//fmt.Println("Light []uint64: ", lightUint)
			//
			//fmt.Println("---------------------------------")
			//
			//start = time.Now()
			//lightTree := merkle.NewTree(sha256.New(), true)
			//for _, l64 := range lightUint {
			//	lightBytes := make([]byte, 8)
			//	binary.LittleEndian.PutUint64(lightBytes, l64)
			//	lightTree.Push(lightBytes)
			//}
			//lhash := lightTree.RootHash()
			//fmt.Println("Light constructing merkle tree: ", "time", time.Since(start))
			//fmt.Printf("Light merkle root hash: %x\n", lhash)
			//
			//fmt.Println("---------------------------------")

			return nil
		},
	}

	return cmd
}
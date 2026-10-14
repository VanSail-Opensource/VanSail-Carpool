package botcommands

import (
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

const nodeBarWidth = 30

var nodeMarkers = []string{"▓", "░", "█", "▒", "▇"}

type displayNode struct {
	Name        string
	CountryCode string
	Bytes       int64
}

func parseBytes(value string) int64 {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed < 0 {
		return 0
	}
	return parsed
}

func formatBytes(value int64) string {
	if value <= 0 {
		return "0 GB"
	}
	units := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	divisor := int64(1)
	unitIndex := 0
	for unitIndex < len(units)-1 && value/divisor >= 1024 {
		divisor *= 1024
		unitIndex++
	}
	if divisor == 1 {
		return fmt.Sprintf("%d B", value)
	}
	whole := value / divisor
	fraction := new(big.Int).Mul(big.NewInt(value%divisor), big.NewInt(100))
	fraction.Quo(fraction, big.NewInt(divisor))
	formatted := fmt.Sprintf("%d.%02d", whole, fraction.Int64())
	formatted = strings.TrimRight(strings.TrimRight(formatted, "0"), ".")
	return formatted + " " + units[unitIndex]
}

func percentage(value, total int64) float64 {
	if value <= 0 || total <= 0 {
		return 0
	}
	return math.Min(100, float64(value)*100/float64(total))
}

func usageBar(percent float64) string {
	filled := int(math.Round(math.Min(100, math.Max(0, percent)) * 8 / 100))
	return strings.Repeat("🟩", filled) + strings.Repeat("⬜", 8-filled)
}

func displayNodes(values []model.TopNode) ([]displayNode, int64) {
	result := make([]displayNode, 0, len(nodeMarkers))
	var total int64
	for _, value := range values {
		bytes := parseBytes(value.TotalBytes)
		if bytes <= 0 {
			continue
		}
		if len(result) == len(nodeMarkers) {
			break
		}
		if bytes > math.MaxInt64-total {
			break
		}
		total += bytes
		result = append(result, displayNode{Name: value.Name, CountryCode: value.CountryCode, Bytes: bytes})
	}
	return result, total
}

func nodeDistribution(nodes []displayNode, total int64) string {
	counts := distributeNodeCells(nodes, total)
	var bar strings.Builder
	for index, count := range counts {
		bar.WriteString(strings.Repeat(nodeMarkers[index], count))
	}
	return bar.String()
}

func distributeNodeCells(nodes []displayNode, total int64) []int {
	counts := make([]int, len(nodes))
	if total <= 0 {
		return counts
	}
	type remainder struct {
		index int
		value float64
	}
	remainders := make([]remainder, len(nodes))
	allocated := 0
	for index, node := range nodes {
		exact := float64(node.Bytes) * nodeBarWidth / float64(total)
		counts[index] = int(math.Floor(exact))
		allocated += counts[index]
		remainders[index] = remainder{index: index, value: exact - float64(counts[index])}
	}
	sort.SliceStable(remainders, func(left, right int) bool { return remainders[left].value > remainders[right].value })
	for index := 0; allocated < nodeBarWidth && index < len(remainders); index++ {
		counts[remainders[index].index]++
		allocated++
	}
	return counts
}

func share(value, total int64) float64 {
	if value <= 0 || total <= 0 {
		return 0
	}
	return float64(value) * 100 / float64(total)
}

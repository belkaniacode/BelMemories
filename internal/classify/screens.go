package classify

// Common monitor and phone screen resolutions (stored landscape).
var screenSizes = map[[2]int]bool{
	// Monitors / laptops
	{1024, 768}: true, {1280, 720}: true, {1280, 800}: true, {1280, 1024}: true,
	{1366, 768}: true, {1440, 900}: true, {1536, 864}: true, {1600, 900}: true,
	{1680, 1050}: true, {1920, 1080}: true, {1920, 1200}: true, {2560, 1080}: true,
	{2560, 1440}: true, {2560, 1600}: true, {2880, 1800}: true, {3440, 1440}: true,
	{3840, 2160}: true, {3024, 1964}: true, {3456, 2234}: true,
	// Phones (portrait sizes normalised to landscape)
	{1600, 720}: true, {2220, 1080}: true,
	{2280, 1080}: true, {2340, 1080}: true, {2400, 1080}: true, {2408, 1080}: true,
	{2412, 1080}: true, {2460, 1080}: true, {2960, 1440}: true,
	{3040, 1440}: true, {3200, 1440}: true, {3120, 1440}: true,
	{1334, 750}: true, {1792, 828}: true, {2436, 1125}: true, {2532, 1170}: true,
	{2556, 1179}: true, {2688, 1242}: true, {2778, 1284}: true, {2796, 1290}: true,
	{2208, 1242}: true, {2622, 1206}: true, {2868, 1320}: true,
}

func isScreenSize(w, h int) bool {
	if w <= 0 || h <= 0 {
		return false
	}
	if h > w {
		w, h = h, w
	}
	return screenSizes[[2]int{w, h}]
}

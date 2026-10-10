package uploads

func publishedThumbnailSize(url string, data []byte) int64 {
	if url == "" {
		return 0
	}
	return int64(len(data))
}

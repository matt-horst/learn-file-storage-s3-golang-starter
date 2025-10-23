package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	jwt, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find jwt", err)
		return
	}

	userID, err := auth.ValidateJWT(jwt, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate jwt", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Couldn't find video", err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Must be video author", err)
		return
	}

	videoFile, videoFileHeaders, err  := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't fild video file", err)
		return
	}
	defer videoFile.Close()

	mediaType, _, err := mime.ParseMediaType(videoFileHeaders.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't determine video file type", err)
		return
	}

	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Invalid video file type", err)
		return
	}

	tmpFileName := "tubely-upload.mp4"
	tmpFile, err := os.CreateTemp("", tmpFileName)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create temporary file", err)
		return
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	const maxUploadSize = 1 << 30

	videoFileReader := http.MaxBytesReader(w, videoFile, maxUploadSize)

	_, err = io.Copy(tmpFile, videoFileReader)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Video file too large", err)
		return
	}

	_, err = tmpFile.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't reset temporary file", err)
		return
	}

	buf := [32]byte{}
	_, err = rand.Read(buf[:])
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create unique id", err)
		return
	}

	_, videoFileExt, ok := strings.Cut(mediaType, "/")
	if !ok {
		respondWithError(w, http.StatusBadRequest, "Invalid media type", err)
	}
 	videoKey := base64.URLEncoding.EncodeToString(buf[:]) + "." + videoFileExt

	params := &s3.PutObjectInput {
		Bucket: &cfg.s3Bucket,
		Key: &videoKey,
		ContentType: &mediaType,
		Body: tmpFile,
	}
	cfg.s3Client.PutObject(r.Context(), params)

	videoURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, videoKey)

	video.VideoURL = &videoURL

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video url", err)
		return
	}
}

package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)


func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}


	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	const maxMemory = 10 << 20
	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't parse video", err)
	}

	file, fileHeader, err := r.FormFile("thumbnail")
	if err !=  nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't retrieve video form file", err)
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't retrieve video info from db", err)
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "You must be video's owner", err)
	}

	mediaType := fileHeader.Header.Get("Content-Type")
	if mediaType == "" {
		respondWithError(w, http.StatusBadRequest, "Missing Content-Type for thumbnail", err)
	}

	fmt.Printf("mediaType: `%s`\n", mediaType)

	_, fileExt, ok := strings.Cut(mediaType, "/")
	if !ok {
		respondWithError(w, http.StatusBadRequest, "Missing file type for thumbnail", err)
	}

	fmt.Printf("ext: `%s`\n", fileExt)
	fileName := videoID.String() + "." + fileExt
	fmt.Printf("name: `%s`\n", fileName)
	filePath := filepath.Join(cfg.assetsRoot, fileName)
	fmt.Printf("path: `%s`\n", filePath)

	localFile, err := os.Create(filePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create file", err)
	}

	io.Copy(localFile, file)

	thumbnailURL := fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, fileName)
	video.ThumbnailURL = &thumbnailURL

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video", err)
	}

	respondWithJSON(w, http.StatusOK, video)
}

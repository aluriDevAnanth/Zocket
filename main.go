package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"mime"

	"github.com/disintegration/imaging"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/lib/pq"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Product struct {
	ID                      uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID                  uint           `json:"user_id"`
	ProductName             string         `json:"product_name"`
	ProductDescription      string         `json:"product_description"`
	ProductImages           pq.StringArray `gorm:"type:text[]" json:"product_images"`
	CompressedProductImages pq.StringArray `gorm:"type:text[]" json:"compressed_product_images"`
	ProductPrice            float64        `json:"product_price"`
}

type User struct {
	ID   uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Name string `json:"name"`
}

var db *gorm.DB
var err error
var logger = logrus.New()

func main() {
	// Setup logger
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetOutput(os.Stdout)
	logger.SetLevel(logrus.InfoLevel)

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	DB_HOST := os.Getenv("DB_HOST")
	DB_USER := os.Getenv("DB_USER")
	DB_PASSWORD := os.Getenv("DB_PASSWORD")
	DB_NAME := os.Getenv("DB_NAME")
	DB_PORT := os.Getenv("DB_PORT")

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		DB_HOST, DB_USER, DB_PASSWORD, DB_NAME, DB_PORT)

	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		logger.Fatalf("Failed to connect to database: %v", err)
	}

	if err := db.AutoMigrate(&Product{}, &User{}); err != nil {
		logger.Fatalf("Failed to migrate database schema: %v", err)
	}

	router := mux.NewRouter()
	router.HandleFunc("/products", createProduct).Methods("POST")
	router.HandleFunc("/products/{id}", getProduct).Methods("GET")
	router.HandleFunc("/products", getProducts).Methods("GET")

	logger.Info("Server is running on port 8080")
	log.Fatal(http.ListenAndServe(":8080", router))
}

func createProduct(w http.ResponseWriter, r *http.Request) {
	var product Product
	if err := json.NewDecoder(r.Body).Decode(&product); err != nil {
		logger.WithFields(logrus.Fields{
			"error": err,
		}).Error("Invalid request payload")
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// Log request details
	logger.WithFields(logrus.Fields{
		"user_id":      product.UserID,
		"product_name": product.ProductName,
	}).Info("Creating product")

	// Process the images (download and compress them)
	originalImagePaths, compressedImagePaths, err := processImages(product.ProductImages)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"error": err,
		}).Error("Image processing error")
		http.Error(w, "Failed to process images", http.StatusInternalServerError)
		return
	}

	// Assign the image paths to the product
	product.ProductImages = pq.StringArray(originalImagePaths)
	product.CompressedProductImages = pq.StringArray(compressedImagePaths)

	fixFilePaths(&product)

	// Insert the product into the database
	if err := db.Create(&product).Error; err != nil {
		logger.WithFields(logrus.Fields{
			"error": err,
		}).Error("Database error while creating product")
		http.Error(w, "Failed to create product", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(product)

	// Log success
	logger.WithFields(logrus.Fields{
		"id":      product.ID,
		"user_id": product.UserID,
	}).Info("Product created successfully")
}

func getProduct(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	id, _ := strconv.Atoi(params["id"])

	var product Product
	if err := db.First(&product, id).Error; err != nil {
		logger.WithFields(logrus.Fields{
			"product_id": id,
			"error":      err,
		}).Error("Product not found")
		http.Error(w, "Product not found", http.StatusNotFound)
		return
	}

	fixFilePaths(&product)

	// Log successful retrieval
	logger.WithFields(logrus.Fields{
		"id": id,
	}).Info("Product retrieved successfully")

	json.NewEncoder(w).Encode(product)
}

func getProducts(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	minPrice := r.URL.Query().Get("min_price")
	maxPrice := r.URL.Query().Get("max_price")
	name := r.URL.Query().Get("product_name")

	var products []Product
	query := db.Model(&Product{})

	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if minPrice != "" {
		min, _ := strconv.ParseFloat(minPrice, 64)
		query = query.Where("product_price >= ?", min)
	}
	if maxPrice != "" {
		max, _ := strconv.ParseFloat(maxPrice, 64)
		query = query.Where("product_price <= ?", max)
	}
	if name != "" {
		query = query.Where("product_name ILIKE ?", "%"+name+"%")
	}

	if err := query.Find(&products).Error; err != nil {
		logger.WithFields(logrus.Fields{
			"error": err,
		}).Error("Failed to retrieve products")
		http.Error(w, "Failed to retrieve products", http.StatusInternalServerError)
		return
	}

	// Log successful retrieval of products
	logger.WithFields(logrus.Fields{
		"count": len(products),
	}).Info("Products retrieved successfully")

	for i := range products {
		for j := range products[i].ProductImages {
			products[i].ProductImages[j] = strings.ReplaceAll(products[i].ProductImages[j], "\\", "/")
		}
	}

	for i := range products {
		for j := range products[i].CompressedProductImages {
			products[i].CompressedProductImages[j] = strings.ReplaceAll(products[i].CompressedProductImages[j], "\\", "/")
		}
	}

	json.NewEncoder(w).Encode(products)
}

func downloadImage(url string, destPath string) (string, string, error) {
	resp, err := http.Get(url)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"url":   url,
			"error": err,
		}).Error("Failed to download image")
		return "", "", fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	ext, _ := mime.ExtensionsByType(contentType)
	if len(ext) == 0 {
		logger.WithFields(logrus.Fields{
			"url":          url,
			"content_type": contentType,
		}).Error("Failed to detect file extension")
		return "", "", fmt.Errorf("failed to detect file extension for content type: %s", contentType)
	}
	extension := ext[len(ext)-1]

	fileNameRand := strings.ReplaceAll(uuid.New().String(), "-", "")
	randomizedFileName := fmt.Sprintf("%s%s", fileNameRand, extension)
	randomizedCompressedFileName := fmt.Sprintf("%s%s", fileNameRand, extension)

	destPathOrg := filepath.Join("images", randomizedFileName)
	destPathCompressed := filepath.Join("compressed_images", randomizedCompressedFileName)

	outFile, err := os.Create(destPathOrg)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"url":   url,
			"error": err,
		}).Error("Failed to create image file")
		return "", "", fmt.Errorf("failed to create image file: %w", err)
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"url":   url,
			"error": err,
		}).Error("Failed to save image")
		return "", "", fmt.Errorf("failed to save image: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"url":         url,
		"destination": destPathOrg,
	}).Info("Image downloaded successfully")

	return destPathOrg, destPathCompressed, nil
}

func compressImage(inputPath, outputPath string) error {
	img, err := imaging.Open(inputPath)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"input_path": inputPath,
			"error":      err,
		}).Error("Failed to open image for compression")
		return fmt.Errorf("failed to open image: %w", err)
	}

	ext := filepath.Ext(inputPath)
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".gif" {
		logger.WithFields(logrus.Fields{
			"input_path": inputPath,
			"extension":  ext,
		}).Error("Unsupported image format")
		return fmt.Errorf("unsupported image format: %s", ext)
	}

	img = imaging.Resize(img, 800, 0, imaging.Lanczos)

	if ext == ".png" {
		err = imaging.Save(img, outputPath)
	} else {
		err = imaging.Save(img, outputPath, imaging.JPEGQuality(80))
	}

	if err != nil {
		logger.WithFields(logrus.Fields{
			"input_path":  inputPath,
			"output_path": outputPath,
			"error":       err,
		}).Error("Failed to save compressed image")
		return fmt.Errorf("failed to save compressed image: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"input_path":  inputPath,
		"output_path": outputPath,
	}).Info("Image compressed successfully")

	return nil
}

func processImages(imageUrls []string) ([]string, []string, error) {
	var originalImagePaths, compressedImagePaths []string

	os.MkdirAll("images", os.ModePerm)
	os.MkdirAll("compressed_images", os.ModePerm)

	for _, url := range imageUrls {
		originalPath := filepath.Join("images", uuid.New().String())

		originalPath, compressedPath, err := downloadImage(url, originalPath)
		if err != nil {
			return nil, nil, err
		}

		err = compressImage(originalPath, compressedPath)
		if err != nil {
			return nil, nil, err
		}

		originalImagePaths = append(originalImagePaths, originalPath)
		compressedImagePaths = append(compressedImagePaths, compressedPath)
	}

	return originalImagePaths, compressedImagePaths, nil
}

func fixFilePaths(product *Product) {
	for i, path := range product.ProductImages {
		product.ProductImages[i] = strings.ReplaceAll(path, "\\", "/")
	}

	for i, path := range product.CompressedProductImages {
		product.CompressedProductImages[i] = strings.ReplaceAll(path, "\\", "/")
	}
}

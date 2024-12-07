# Zocket Product Management Service

## Overview

This project implements a RESTful API service for managing products and their associated information, such as user ownership, pricing, and images (both original and compressed versions). It leverages the following technologies:

- Go for server-side programming.
- GORM as the ORM library for database interactions.
- PostgreSQL as the database.
- Gorilla Mux for HTTP routing.
- Disintegration Imaging for image processing.
- Logrus for structured logging.
- UUID for generating unique file names for uploaded images.

## Architectural Choices

### 1. **Database Design**

The schema uses two primary tables:

- **Users**: Holds user data (`ID`, `Name`).
- **Products**: Stores product details, including `UserID`, `ProductName`, `ProductDescription`, `ProductImages`, `CompressedProductImages`, and `ProductPrice`. The `ProductImages` and `CompressedProductImages` fields utilize the `pq.StringArray` type to store lists of image paths.

### 2. **Image Handling**

- Images are downloaded from provided URLs, saved locally, and then compressed.
- Compression uses the `Disintegration Imaging` library to ensure fast and high-quality resizing.
- Separate folders (`images/` and `compressed_images/`) are used to organize original and compressed images.
- File names are randomized using UUIDs to avoid collisions and maintain security.

### 3. **Logging**

- Logs are structured using `Logrus` with fields like error details, request parameters, and process status. This provides better insights during debugging and monitoring.

### 4. **Routing**

The REST API endpoints are managed using `Gorilla Mux`. Routes are defined for creating, fetching, and querying products.

## API Endpoints

### 1. **Create Product**

- **URL**: `/products`
- **Method**: `POST`
- **Request Payload:**

  ```{
  "user_id": 1,
  "product_name": "Laptop",
  "product_description": "A powerful gaming laptop.",
  "product_images": ["https://example.com/image1.jpg"],
  "product_price": 1200.99
  }
  ```

- **Response**:

  ```{
  "id": 1,
  "user_id": 1,
  "product_name": "Laptop",
  "product_description": "A powerful gaming laptop.",
  "product_images": ["/images/<uuid>.jpg"],
  "compressed_product_images": ["/compressed_images/<uuid>.jpg"],
  "product_price": 1200.99
  }
  ```

### Get Product by ID

- **URL**: `/products/{id}`
- **Method**: `GET`
- **Response**:

  ```
  {
      "id": 1,
      "user_id": 1,
      "product_name": "Laptop",
      "product_description": "A powerful gaming laptop.",
      "product_images": ["/images/<uuid>.jpg"],
      "compressed_product_images": ["/compressed_images/<uuid>.jpg"],
      "product_price": 1200.99
  }

  ```

  ### 3. **Get Products with Filters**

  - **URL**: `/products`
  - **Method**: `GET`
  - **Query Parameters**:

    - `user_id`: Filter by user ID.
    - `min_price`: Minimum price filter.
    - `max_price`: Maximum price filter.
    - `product_name`: Case-insensitive name filter

  - ```
    [
        {
            "id": 1,
            "user_id": 1,
            "product_name": "Laptop",
            "product_description": "A powerful gaming laptop.",
            "product_images": ["/images/<uuid>.jpg"],
            "compressed_product_images": ["/compressed_images/<uuid>.jpg"],
            "product_price": 1200.99
        }
    ]
    ```

## Setup Instructions

### 1. **Prerequisites**

- Install Go (1.19 or later).
- Install PostgreSQL.
- Set up `imaging` and other dependencies:

  ```go
  go get github.com/gorilla/mux
  go get github.com/sirupsen/logrus
  go get github.com/lib/pq
  go get github.com/google/uuid
  ```

### 2. **Database Configuration**

1. Create a PostgreSQL database:

   ```
   CREATE DATABASE test;
   ```

2. ```
   host=localhost user=postgres password=<password> dbname=test port=5432 sslmode=disable
   ```

### 3. **Run the Application**

1. Build and run the server:

   ```
   go run main.go
   ```

2. The server starts on `http://localhost:8080`.

### 4. **Image Storage**

- Ensure the `images/` and `compressed_images/` directories exist with write permissions.

  ```
  mkdir images compressed_images
  chmod 755 images compressed_images
  ```

## Assumptions

1. **Image URLs**: The system assumes valid and accessible image URLs are provided in the payload.
2. **Compression Logic**: Images larger than 800px in width are resized proportionally, with a default quality of 80% for JPEG.
3. **Data Storage**: Only file paths for images are stored in the database. The actual files are saved on the local filesystem.

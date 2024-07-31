package main

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"image"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aybabtme/rgbterm"
	"github.com/golang-jwt/jwt/v5"
	"github.com/nfnt/resize"
)

type BoxConfig struct {
	ClientID           string
	ClientSecret       string
	EnterpriseID       string
	PrivateKey         []byte
	PrivateKeyPassword string
	PublicKeyID        string
}

type BoxClient struct {
	config     BoxConfig
	token      string
	privateKey *rsa.PrivateKey
}

type ConfigFile struct {
	BoxAppSettings BoxAppSettings `json:"boxAppSettings"`
	EnterpriseID   string         `json:"enterpriseID"`
}

type BoxAppSettings struct {
	ClientID     string  `json:"clientID"`
	ClientSecret string  `json:"clientSecret"`
	AppAuth      AppAuth `json:"appAuth"`
}

type AppAuth struct {
	KeyID      string        `json:"keyID"`
	PrivateKey CleanedString `json:"privateKey"`
	Passphrase string        `json:"passphrase"`
}

type CleanedString string

func parsePrivateKey(privateKeyPEM []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(privateKeyPEM)
	if block == nil {
		return nil, errors.New("failed to parse PEM block containing the private key")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err == nil {
		return privateKey, nil
	}

	parsedKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err == nil {
		privateKey, ok := parsedKey.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("key is not an RSA private key")
		}
		return privateKey, nil
	}

	return nil, errors.New("failed to parse private key as PKCS1 or PKCS8")
}

func NewBoxClient(config BoxConfig) (*BoxClient, error) {
	// Ensure the private key is in PEM format
	privateKeyPEM := []byte(config.PrivateKey)
	if len(privateKeyPEM) == 0 {
		return nil, errors.New("private key is empty")
	}
	if !bytes.HasPrefix(privateKeyPEM, []byte("-----BEGIN")) {
		privateKeyPEM = []byte("-----BEGIN PRIVATE KEY-----\n" + string(config.PrivateKey) + "\n-----END PRIVATE KEY-----")
	}

	// Parse the private key
	privateKey, err := parsePrivateKey(privateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %v", err)
	}

	// Create a new BoxClient with the parsed private key
	client := &BoxClient{
		config:     config,
		privateKey: privateKey,
	}

	// Authenticate the client
	if err := client.authenticate(); err != nil {
		return nil, err
	}

	return client, nil
}

func (c *BoxClient) authenticate() error {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss":          c.config.ClientID,
		"sub":          c.config.EnterpriseID,
		"box_sub_type": "enterprise",
		"aud":          "https://api.box.com/oauth2/token",
		"jti":          fmt.Sprintf("%d", time.Now().UnixNano()),
		"exp":          time.Now().Add(time.Minute * 45).Unix(),
	})

	token.Header["kid"] = c.config.PublicKeyID

	signedToken, err := token.SignedString(c.privateKey)
	if err != nil {
		return fmt.Errorf("failed to sign JWT: %v", err)
	}

	// Rest of the function remains the same
	resp, err := http.PostForm("https://api.box.com/oauth2/token", url.Values{
		"grant_type":    {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"client_id":     {c.config.ClientID},
		"client_secret": {c.config.ClientSecret},
		"assertion":     {signedToken},
	})
	if err != nil {
		return fmt.Errorf("failed to get access token: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %v", err)
	}

	c.token = result.AccessToken
	return nil
}

func (c *BoxClient) getImagesFromFolder(folderID string) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("https://api.box.com/2.0/folders/%s/items?fields=id,name,extension", folderID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Authorization", "Bearer "+c.token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get folder items: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Entries []map[string]interface{} `json:"entries"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	var images []map[string]interface{}
	for _, item := range result.Entries {
		if ext, ok := item["extension"].(string); ok {
			if ext == "jpg" || ext == "jpeg" || ext == "png" || ext == "gif" {
				images = append(images, item)
			}
		}
	}

	return images, nil
}

func getRandomImage(images []map[string]interface{}) map[string]interface{} {
	rand.Seed(time.Now().UnixNano())
	return images[rand.Intn(len(images))]
}

func (c *BoxClient) downloadImage(fileID string) ([]byte, error) {
	url := fmt.Sprintf("https://api.box.com/2.0/files/%s/content", fileID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Authorization", "Bearer "+c.token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %v", err)
	}
	defer resp.Body.Close()

	return ioutil.ReadAll(resp.Body)
}

func imageToASCII(imgData []byte, width int) (string, error) {
	img, _, err := image.Decode(bytes.NewReader(imgData))
	if err != nil {
		return "", fmt.Errorf("failed to decode image: %v", err)
	}

	img = resize.Resize(uint(width), 0, img, resize.Lanczos3)
	bounds := img.Bounds()
	w, h := bounds.Max.X, bounds.Max.Y

	var result strings.Builder
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			avg := (r + g + b) / 3
			char := getASCIIChar(avg)
			result.WriteString(rgbterm.FgString(string(char), uint8(r>>8), uint8(g>>8), uint8(b>>8)))
		}
		result.WriteString("\n")
	}

	return result.String(), nil
}

func getASCIIChar(avg uint32) byte {
	chars := []byte(" .:-=+*#%@")
	index := int(avg * uint32(len(chars)-1) / 65535)
	return chars[index]
}

func (cs *CleanedString) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*cs = CleanedString(strings.ReplaceAll(s, "\n", ""))
	return nil
}

func main() {

	configFile, err := os.Open("./internal/private/config.json")
	if err != nil {
		fmt.Println("Error in opening config file: ", err)
	}

	byteValue, err := ioutil.ReadAll(configFile)
	if err != nil {
		fmt.Println("Error getting bytes from file: ", err)
	}
	defer configFile.Close()

	var result *ConfigFile
	json.Unmarshal([]byte(byteValue), &result)

	config := BoxConfig{
		ClientID:           result.BoxAppSettings.ClientID,
		ClientSecret:       result.BoxAppSettings.ClientSecret,
		EnterpriseID:       result.EnterpriseID,
		PrivateKey:         []byte(result.BoxAppSettings.AppAuth.PrivateKey),
		PrivateKeyPassword: result.BoxAppSettings.AppAuth.Passphrase,
		PublicKeyID:        result.BoxAppSettings.AppAuth.KeyID,
	}

	client, err := NewBoxClient(config)
	if err != nil {
		log.Fatalf("Failed to create Box client: %v", err)
	}

	folderID := "YOUR_FOLDER_ID"
	images, err := client.getImagesFromFolder(folderID)
	if err != nil {
		log.Fatalf("Failed to get images: %v", err)
	}

	if len(images) == 0 {
		log.Fatal("No images found in the specified folder")
	}

	randomImage := getRandomImage(images)
	imgData, err := client.downloadImage(randomImage["id"].(string))
	if err != nil {
		log.Fatalf("Failed to download image: %v", err)
	}

	asciiArt, err := imageToASCII(imgData, 80) // Adjust width as needed
	if err != nil {
		log.Fatalf("Failed to convert image to ASCII: %v", err)
	}

	fmt.Println(asciiArt)
}

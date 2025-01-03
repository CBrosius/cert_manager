package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func createRootCertificate(c *gin.Context) {
	commonName := c.PostForm("common_name")
	organization := c.PostForm("organization")
	organizationalUnit := c.PostForm("organizational_unit")
	country := c.PostForm("country")
	state := c.PostForm("state")
	locality := c.PostForm("locality")
	email := c.PostForm("email")
	validityYearsStr := c.PostForm("validity_years")

	if commonName == "" || validityYearsStr == "" {
		c.String(http.StatusBadRequest, "Common Name and Validity Period are mandatory fields.")
		return
	}

	validityYears, err := strconv.Atoi(validityYearsStr)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid validity period: %v", err)
		return
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error generating private key: %v", err)
		return
	}

	rootCertTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:         commonName,
			Organization:       []string{organization},
			OrganizationalUnit: []string{organizationalUnit},
			Country:            []string{country},
			Province:           []string{state},
			Locality:           []string{locality},
			ExtraNames: []pkix.AttributeTypeAndValue{
				{
					Type:  []int{1, 2, 840, 113549, 1, 9, 1},
					Value: email,
				},
			},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(validityYears, 0, 0),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}

	rootCertBytes, err := x509.CreateCertificate(rand.Reader, rootCertTemplate, rootCertTemplate, &privateKey.PublicKey, privateKey)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error creating root certificate: %v", err)
		return
	}

	os.MkdirAll("data/root-cert", os.ModePerm)

	rootCertFilename := "data/root-cert/HomeLab_Root_CA.pem"
	rootKeyFilename := "data/root-cert/HomeLab_Root_CA.key"

	rootCertFile, err := os.Create(rootCertFilename)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error creating root certificate file: %v", err)
		return
	}
	defer rootCertFile.Close()
	pem.Encode(rootCertFile, &pem.Block{Type: "CERTIFICATE", Bytes: rootCertBytes})

	rootKeyFile, err := os.Create(rootKeyFilename)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error creating root key file: %v", err)
		return
	}
	defer rootKeyFile.Close()
	pem.Encode(rootKeyFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})

	// Save optional fields to settings.json
	viper.Set("general_cert_options.organization", strings.TrimSpace(organization))
	viper.Set("general_cert_options.organization_unit", strings.TrimSpace(organizationalUnit))
	viper.Set("general_cert_options.country", strings.TrimSpace(country))
	viper.Set("general_cert_options.state", strings.TrimSpace(state))
	viper.Set("general_cert_options.location", strings.TrimSpace(locality))
	viper.Set("general_cert_options.email", strings.TrimSpace(email))
	if err := viper.WriteConfig(); err != nil {
		c.String(http.StatusInternalServerError, "Error saving settings: %v", err)
		return
	}

	c.Redirect(http.StatusSeeOther, "/certificates")
}

func deleteRootCertificate(c *gin.Context) {
	fileName := c.Param("filename")
	filePathPem := "./data/root-cert/" + fileName
	filePathKey := strings.TrimSuffix(filePathPem, ".pem") + ".key"

	// Remove the .pem file
	err := os.Remove(filePathPem)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error deleting root certificate .pem file: %v", err)
		return
	}

	// Remove the .key file
	err = os.Remove(filePathKey)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error deleting root certificate .key file: %v", err)
		return
	}

	// Remove all files in the certs folder
	certFiles, err := os.ReadDir("./data/certs")
	if err != nil {
		c.String(http.StatusInternalServerError, "Error reading certificates directory: %v", err)
		return
	}

	for _, file := range certFiles {
		err = os.Remove("./data/certs/" + file.Name())
		if err != nil {
			c.String(http.StatusInternalServerError, "Error deleting certificate file: %v", err)
			return
		}
	}

	// Redirect to the index page to create a new root certificate
	c.Redirect(http.StatusSeeOther, "/")
}

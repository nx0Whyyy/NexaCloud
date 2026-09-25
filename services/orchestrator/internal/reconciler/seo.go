package reconciler

import (
	"encoding/json"
	"html"
	"net/http"
	"strings"
)

const cloudURL = "https://cloud.nexastudio.dev"

type seoPage struct {
	Title       string
	Description string
	Image       string
	Index       bool
	Schema      any
}

var seoPages = map[string]seoPage{
	"/":               {Title: "NexaCloud - Manage Your Minecraft Infrastructure", Description: "Manage Paper servers, Docker nodes, monitoring and Minecraft infrastructure from one secure NexaCloud control plane.", Image: "/assets/nexacloud-network-v2.png", Index: true, Schema: map[string]any{"@context": "https://schema.org", "@type": "SoftwareApplication", "name": "NexaCloud", "url": cloudURL, "applicationCategory": "DeveloperApplication", "operatingSystem": "Linux", "description": "Minecraft infrastructure management platform for Paper servers and Docker nodes.", "offers": map[string]any{"@type": "Offer", "price": "0", "priceCurrency": "EUR", "name": "Free"}}},
	"/platform":       {Title: "NexaCloud Features - Minecraft Infrastructure Automation", Description: "Explore NexaCloud orchestration, node management, service discovery, recovery and monitoring for modern Minecraft networks.", Image: "/assets/nexacloud-network-v2.png", Index: true},
	"/infrastructure": {Title: "Minecraft Multi-Node Infrastructure | NexaCloud", Description: "Understand the NexaCloud control plane, NexaAgent, scheduling and event-driven architecture for distributed Minecraft infrastructure.", Image: "/assets/nexacloud-world.png", Index: true},
	"/pricing":        {Title: "NexaCloud Pricing - Plans for Minecraft Networks", Description: "Start with the NexaCloud Free plan: one Minecraft network, one Docker node and up to three active server instances.", Image: "/assets/nexacloud-network-v2.png", Index: true},
	"/security":       {Title: "NexaCloud Security - Infrastructure and Account Protection", Description: "Learn how NexaCloud protects accounts, node enrollment, server files, sessions and infrastructure operations.", Image: "/assets/nexacloud-world.png", Index: true},
	"/download":       {Title: "Download NexaAgent | NexaCloud", Description: "Install NexaAgent on a Linux Docker node and securely connect your Minecraft server infrastructure to NexaCloud.", Image: "/assets/nexacloud-network-v2.png", Index: true},
	"/roadmap":        {Title: "NexaCloud Roadmap - Minecraft Infrastructure Platform", Description: "Follow the current NexaCloud foundation and the planned work for deployments, backups, orchestration and network automation.", Image: "/assets/nexacloud-world.png", Index: true},
	"/docs":           {Title: "NexaCloud Documentation - Install and Manage Your Network", Description: "Learn how to install NexaAgent, register Docker nodes and operate Minecraft servers through the NexaCloud control plane.", Image: "/assets/nexacloud-network-v2.png", Index: true},
	"/login":          {Title: "Sign In | NexaCloud", Description: "Sign in to the NexaCloud control plane.", Index: false},
	"/register":       {Title: "Create Your NexaCloud Account", Description: "Create a NexaCloud account and connect your first Minecraft infrastructure node.", Index: false},
	"/verify":         {Title: "Verify Your Email | NexaCloud", Description: "Verify your NexaCloud account email.", Index: false},
}

func metadata(path string, page seoPage) string {
	robots := "noindex,nofollow"
	if page.Index {
		robots = "index,follow,max-image-preview:large"
	}
	image := page.Image
	if image == "" {
		image = "/assets/nexacloud-network-v2.png"
	}
	canonical := cloudURL + path
	tags := `<meta name="description" content="` + html.EscapeString(page.Description) + `"><meta name="robots" content="` + robots + `"><link rel="canonical" href="` + canonical + `"><meta property="og:site_name" content="NexaCloud"><meta property="og:type" content="website"><meta property="og:title" content="` + html.EscapeString(page.Title) + `"><meta property="og:description" content="` + html.EscapeString(page.Description) + `"><meta property="og:url" content="` + canonical + `"><meta property="og:image" content="` + cloudURL + image + `"><meta name="twitter:card" content="summary_large_image"><meta name="twitter:title" content="` + html.EscapeString(page.Title) + `"><meta name="twitter:description" content="` + html.EscapeString(page.Description) + `"><meta name="twitter:image" content="` + cloudURL + image + `"><meta name="theme-color" content="#080d0c"><link rel="manifest" href="/site.webmanifest">`
	if page.Schema != nil {
		data, _ := json.Marshal(page.Schema)
		tags += `<script type="application/ld+json">` + string(data) + `</script>`
	}
	return tags
}

func renderSEOPage(source, path string, page seoPage) string {
	source = strings.Replace(source, `<meta name="description" content="NexaCloud orchestre, déploie et supervise votre réseau Minecraft depuis un control plane unique.">`, "", 1)
	source = strings.Replace(source, `<meta name="theme-color" content="#080d0c">`, "", 1)
	start := strings.Index(source, "<title>")
	end := strings.Index(source, "</title>")
	if start >= 0 && end > start {
		source = source[:start] + "<title>" + html.EscapeString(page.Title) + "</title>" + source[end+8:]
	}
	return strings.Replace(source, "</head>", metadata(path, page)+"</head>", 1)
}

func serveText(contentType, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_, _ = w.Write([]byte(body))
	}
}

func robotsText() string {
	return "User-agent: *\nAllow: /\nDisallow: /api/\nDisallow: /dashboard\nDisallow: /staff\nDisallow: /login\nDisallow: /register\nDisallow: /verify\n\nSitemap: " + cloudURL + "/sitemap.xml\n"
}

func sitemapXML() string {
	paths := []string{"/", "/platform", "/infrastructure", "/pricing", "/security", "/download", "/roadmap", "/docs"}
	var body strings.Builder
	body.WriteString(`<?xml version="1.0" encoding="UTF-8"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`)
	for _, path := range paths {
		body.WriteString("<url><loc>" + cloudURL + path + "</loc></url>")
	}
	body.WriteString("</urlset>")
	return body.String()
}

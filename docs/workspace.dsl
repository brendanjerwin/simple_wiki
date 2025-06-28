workspace {
    !identifiers hierarchical

    model {
        user = person "User" "A person using the wiki."

        wikiSystem = softwareSystem "Simple Wiki" "A simple wiki application." {
            goProcess = container "GoProcess" "The single Go process that runs the wiki." "Go" {
                //DO NOT nest component declarations. That does not work.

                cli = component "Command Line Interface" "Parses flags and bootstraps the application." "Go (urfave/cli)" {
                    properties {
                        file "main.go"
                      }
                  }
                multiplexer = component "Request Multiplexer" "Routes incoming HTTP traffic to either gRPC or the Web Server based on content-type." "Go (net/http)" {
                    properties {
                        file "main.go"
                    }
                }

                ginWebServer = component "Gin Web Server" "Handles all non-gRPC HTTP traffic." "Gin" {
                    properties {
                        file "internal/web/server.go"
                    }
                }

                webHandlers = component "Web UI Handlers" "Handles requests for viewing and editing pages." "Go" {
                    properties {
                        file "internal/web/handlers.go"
                    }
                }
                apiHandlers = component "REST API Handlers" "Handles API requests for search and frontmatter queries." "Go" {
                    properties {
                        file "internal/api/handlers.go"
                    }
                }
                sessionManager = component "Session Manager" "Manages user session cookies." "Go (gin-contrib/sessions)" {
                    properties {
                        file "internal/web/server.go"
                    }
                }
                authMiddleware = component "Auth Middleware" "Protects the site with a secret code." "Go" {
                    properties {
                        file "internal/web/auth.go"
                    }
                }

                grpcServer = component "gRPC Server" "Handles all gRPC API requests." "gRPC" {
                    properties {
                        file "internal/grpc/server.go"
                    }
                }
                versionService = component "Version Service" "Provides the GetVersion RPC." {
                    properties {
                        file "internal/grpc/version/service.go"
                    }
                }
                frontmatterService = component "Frontmatter Service" "Provides RPCs for manipulating page frontmatter." {
                    properties {
                        file "internal/grpc/frontmatter/service.go"
                    }
                }

                pageManager = component "Page Manager" "Manages the lifecycle of wiki pages (CRUD)." "Go" {
                    properties {
                        file "internal/page/manager.go"
                    }
                }

                indexer = component "Indexer" "Manages all search indexes." {
                    properties {
                        file "internal/indexer/indexer.go"
                    }
                }
                multiMaintainer = component "Multi-Maintainer" "Coordinates all registered indexers." "Go" {
                    properties {
                        file "internal/indexer/multi.go"
                    }
                }
                bleveIndex = component "Bleve Index" "Full-text search index for page content." "Bleve" {
                    properties {
                        file "internal/indexer/bleve.go"
                    }
                }
                frontmatterIndex = component "Frontmatter Index" "Index for querying structured data in frontmatter." "Go" {
                    properties {
                        file "internal/indexer/frontmatter.go"
                    }
                }
                markdownRenderer = component "Markdown Renderer" "Converts Markdown text to HTML." "Goldmark" {
                    properties {
                        file "internal/markdown/renderer.go"
                    }
                }
                templateEngine = component "Template Engine" "Executes Go templates for dynamic content." "Go" {
                    properties {
                        file "internal/template/engine.go"
                    }
                }

                labelPrinterClient = component "Label Printer Client" "Client for interacting with external label printers." {
                    properties {
                        file "internal/labelprinter/client.go"
                    }
                }
                usbDriver = component "USB Printer Driver" "Communicates with direct-connect USB printers." {
                    properties {
                        file "internal/labelprinter/usb.go"
                    }
                }
                lpDriver = component "LP Printer Driver" "Uses the 'lp' command-line tool to print." {
                    properties {
                        file "internal/labelprinter/lp.go"
                    }
                }
            }
        }

        fileSystem = softwareSystem "File System" "Stores page content and history as .md and .json files." "External System"
        labelPrinter = softwareSystem "Label Printer" "An external USB or network-connected label printer." "External System"

        // Relationships
        user -> wikiSystem.goProcess.multiplexer "Makes HTTP requests"
        wikiSystem.goProcess.cli -> wikiSystem.goProcess.multiplexer "Starts"

        wikiSystem.goProcess.multiplexer -> wikiSystem.goProcess.ginWebServer "Forwards Web/API traffic"
        wikiSystem.goProcess.multiplexer -> wikiSystem.goProcess.grpcServer "Forwards gRPC traffic"

        wikiSystem.goProcess.ginWebServer -> wikiSystem.goProcess.sessionManager "Uses"
        wikiSystem.goProcess.ginWebServer -> wikiSystem.goProcess.authMiddleware "Uses"
        wikiSystem.goProcess.ginWebServer -> wikiSystem.goProcess.webHandlers "Routes to"
        wikiSystem.goProcess.ginWebServer -> wikiSystem.goProcess.apiHandlers "Routes to"
        wikiSystem.goProcess.grpcServer -> wikiSystem.goProcess.frontmatterService "Routes to"
        wikiSystem.goProcess.grpcServer -> wikiSystem.goProcess.versionService "Routes to"

        wikiSystem.goProcess.webHandlers -> wikiSystem.goProcess.pageManager
        wikiSystem.goProcess.apiHandlers -> wikiSystem.goProcess.indexer
        wikiSystem.goProcess.apiHandlers -> wikiSystem.goProcess.labelPrinterClient

        wikiSystem.goProcess.frontmatterService -> wikiSystem.goProcess.pageManager

        wikiSystem.goProcess.pageManager -> fileSystem "Reads/Writes page files"
        wikiSystem.goProcess.pageManager -> wikiSystem.goProcess.indexer "Updates index on change"
        wikiSystem.goProcess.pageManager -> wikiSystem.goProcess.markdownRenderer "Renders markdown"
        wikiSystem.goProcess.pageManager -> wikiSystem.goProcess.templateEngine "Renders templates"

        wikiSystem.goProcess.multiMaintainer -> wikiSystem.goProcess.bleveIndex "Maintains"
        wikiSystem.goProcess.multiMaintainer -> wikiSystem.goProcess.frontmatterIndex "Maintains"

        wikiSystem.goProcess.labelPrinterClient -> labelPrinter "Sends ZPL data"
    }

    views {
        systemContext wikiSystem "SystemContext" {
            include *
            autoLayout
        }

        container wikiSystem "Containers" {
            include *
            autoLayout
        }

        // Component Views
        component wikiSystem.goProcess "Components" {
            title "Simple Wiki Components"
            include *
            autoLayout
        }

        styles {
            element "Software System" {
                background #1168bd
                color #ffffff
            }
            element "Container" {
                background #438dd5
                color #ffffff
            }
            element "Component" {
                background #85bbf0
                color #000000
            }
            element "Person" {
                background #08427b
                color #ffffff
            }
        }
    }
}

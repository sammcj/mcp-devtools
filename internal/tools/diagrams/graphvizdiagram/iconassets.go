package graphvizdiagram

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed assets/aws/Architecture-Group/*.png
//go:embed assets/aws/Architecture-Service/*/*.png
//go:embed assets/aws/Category/*.png
//go:embed assets/aws/Resource/*/*.png
//go:embed assets/aws/Resource/*/*/*.png
var embeddedAssets embed.FS

// IconAsset represents an icon with its path and metadata
type IconAsset struct {
	Path     string // Relative path from assets directory
	Label    string // Display label
	Category string // Category (compute, database, etc.)
	Provider string // Provider (aws, gcp, etc.)
}

// tempIconCache stores extracted icon paths during runtime
var tempIconCache = make(map[string]string)
var tempIconDir string

// initTempIconDir creates a temporary directory for extracted icons
func initTempIconDir() error {
	if tempIconDir != "" {
		return nil // Already initialized
	}

	dir, err := os.MkdirTemp("", "graphviz-icons-*")
	if err != nil {
		return fmt.Errorf("failed to create temp icon directory: %w", err)
	}
	tempIconDir = dir
	return nil
}

// extractIconToTemp extracts an embedded icon to a temporary file
func extractIconToTemp(embeddedPath string) (string, error) {
	// Check cache first
	if cachedPath, exists := tempIconCache[embeddedPath]; exists {
		return cachedPath, nil
	}

	// Initialize temp directory if needed
	if err := initTempIconDir(); err != nil {
		return "", err
	}

	// Read embedded file
	data, err := embeddedAssets.ReadFile(embeddedPath)
	if err != nil {
		return "", fmt.Errorf("failed to read embedded icon %s: %w", embeddedPath, err)
	}

	// Create temp file path maintaining directory structure
	relativePath := strings.TrimPrefix(embeddedPath, "assets/")
	tempPath := filepath.Join(tempIconDir, relativePath)

	// Create directory structure
	if err := os.MkdirAll(filepath.Dir(tempPath), 0700); err != nil {
		return "", fmt.Errorf("failed to create icon directory: %w", err)
	}

	// Write to temp file
	if err := os.WriteFile(tempPath, data, 0600); err != nil {
		return "", fmt.Errorf("failed to write icon to temp: %w", err)
	}

	// Cache the path
	tempIconCache[embeddedPath] = tempPath
	return tempPath, nil
}

// getIconPath returns the full path to an icon file given a type like "aws.ec2"
func (t *GraphvizDiagramTool) getIconPath(nodeType string) (string, bool) {
	// Map common type names to actual icon files
	iconMap := t.getIconPathMap()

	if iconPath, exists := iconMap[nodeType]; exists {
		// Build the embedded asset path
		embeddedPath := "assets/" + iconPath

		// Check if the icon exists in embedded assets
		if _, err := fs.Stat(embeddedAssets, embeddedPath); err == nil {
			// Extract to temp and return the temp path
			tempPath, err := extractIconToTemp(embeddedPath)
			if err == nil {
				return tempPath, true
			}
		}
	}

	return "", false
}

// getIconPathMap returns a mapping of logical names to actual icon file paths
func (t *GraphvizDiagramTool) getIconPathMap() map[string]string {
	return map[string]string{
		// AWS Architecture Service Icons
		"aws.ec2":     "aws/Architecture-Service/Compute/EC2.png",
		"aws.lambda":  "aws/Architecture-Service/Compute/Lambda.png",
		"aws.ecs":     "aws/Architecture-Service/Containers/Elastic-Container-Service.png",
		"aws.eks":     "aws/Architecture-Service/Containers/Elastic-Kubernetes-Service.png",
		"aws.fargate": "aws/Architecture-Service/Compute/Fargate.png",
		"aws.batch":   "aws/Architecture-Service/Compute/Batch.png",

		// AWS Database
		"aws.rds":         "aws/Architecture-Service/Database/RDS.png",
		"aws.aurora":      "aws/Architecture-Service/Database/Aurora.png",
		"aws.dynamodb":    "aws/Architecture-Service/Database/DynamoDB.png",
		"aws.elasticache": "aws/Architecture-Service/Database/ElastiCache.png",
		"aws.documentdb":  "aws/Architecture-Service/Database/DocumentDB.png",
		"aws.neptune":     "aws/Architecture-Service/Database/Neptune.png",
		"aws.timestream":  "aws/Architecture-Service/Database/Timestream.png",
		"aws.memorydb":    "aws/Architecture-Service/Database/MemoryDB.png",
		"aws.keyspaces":   "aws/Architecture-Service/Database/Keyspaces.png",

		// AWS Storage
		"aws.s3":              "aws/Architecture-Service/Storage/Simple-Storage-Service.png",
		"aws.efs":             "aws/Architecture-Service/Storage/Elastic-File-System.png",
		"aws.fsx":             "aws/Architecture-Service/Storage/FSx.png",
		"aws.backup":          "aws/Architecture-Service/Storage/Backup.png",
		"aws.storage-gateway": "aws/Architecture-Service/Storage/Storage-Gateway.png",

		// AWS Networking
		"aws.vpc":           "aws/Architecture-Group/Virtual-private-cloud-VPC.png",
		"aws.cloudfront":    "aws/Architecture-Service/Networking-Content-Delivery/CloudFront.png",
		"aws.route53":       "aws/Architecture-Service/Networking-Content-Delivery/Route-53.png",
		"aws.apigateway":    "aws/Architecture-Service/Networking-Content-Delivery/API-Gateway.png",
		"aws.elb":           "aws/Architecture-Service/Networking-Content-Delivery/Elastic-Load-Balancing.png",
		"aws.alb":           "aws/Architecture-Service/Networking-Content-Delivery/Elastic-Load-Balancing.png",
		"aws.nlb":           "aws/Architecture-Service/Networking-Content-Delivery/Elastic-Load-Balancing.png",
		"aws.directconnect": "aws/Architecture-Service/Networking-Content-Delivery/Direct-Connect.png",
		"aws.vpn":           "aws/Architecture-Service/Networking-Content-Delivery/Site-to-Site-VPN.png",
		"aws.privatelink":   "aws/Architecture-Service/Networking-Content-Delivery/PrivateLink.png",

		// AWS Application Integration
		"aws.sqs":           "aws/Architecture-Service/App-Integration/Simple-Queue-Service.png",
		"aws.sns":           "aws/Architecture-Service/App-Integration/Simple-Notification-Service.png",
		"aws.eventbridge":   "aws/Architecture-Service/App-Integration/EventBridge.png",
		"aws.appsync":       "aws/Architecture-Service/App-Integration/AppSync.png",
		"aws.stepfunctions": "aws/Architecture-Service/App-Integration/Step-Functions.png",
		"aws.mq":            "aws/Architecture-Service/App-Integration/MQ.png",

		// AWS Analytics
		"aws.athena":     "aws/Architecture-Service/Analytics/Athena.png",
		"aws.emr":        "aws/Architecture-Service/Analytics/EMR.png",
		"aws.kinesis":    "aws/Architecture-Service/Analytics/Kinesis.png",
		"aws.glue":       "aws/Architecture-Service/Analytics/Glue.png",
		"aws.redshift":   "aws/Architecture-Service/Analytics/Redshift.png",
		"aws.quicksight": "aws/Architecture-Service/Analytics/QuickSight.png",
		"aws.opensearch": "aws/Architecture-Service/Analytics/OpenSearch-Service.png",
		"aws.msk":        "aws/Architecture-Service/Analytics/Managed-Streaming-for-Apache-Kafka.png",
		"aws.datazone":   "aws/Architecture-Service/Analytics/DataZone.png",

		// AWS AI/ML
		"aws.sagemaker":   "aws/Architecture-Service/Artificial-Intelligence/SageMaker.png",
		"aws.bedrock":     "aws/Architecture-Service/Artificial-Intelligence/Bedrock.png",
		"aws.comprehend":  "aws/Architecture-Service/Artificial-Intelligence/Comprehend.png",
		"aws.rekognition": "aws/Architecture-Service/Artificial-Intelligence/Rekognition.png",
		"aws.polly":       "aws/Architecture-Service/Artificial-Intelligence/Polly.png",
		"aws.translate":   "aws/Architecture-Service/Artificial-Intelligence/Translate.png",
		"aws.transcribe":  "aws/Architecture-Service/Artificial-Intelligence/Transcribe.png",
		"aws.lex":         "aws/Architecture-Service/Artificial-Intelligence/Lex.png",
		"aws.forecast":    "aws/Architecture-Service/Artificial-Intelligence/Forecast.png",
		"aws.personalize": "aws/Architecture-Service/Artificial-Intelligence/Personalize.png",
		"aws.textract":    "aws/Architecture-Service/Artificial-Intelligence/Textract.png",

		// AWS Security
		"aws.iam":       "aws/Architecture-Service/Security-Identity-Compliance/Identity-and-Access-Management.png",
		"aws.cognito":   "aws/Architecture-Service/Security-Identity-Compliance/Cognito.png",
		"aws.secrets":   "aws/Architecture-Service/Security-Identity-Compliance/Secrets-Manager.png",
		"aws.kms":       "aws/Architecture-Service/Security-Identity-Compliance/Key-Management-Service.png",
		"aws.acm":       "aws/Architecture-Service/Security-Identity-Compliance/Certificate-Manager.png",
		"aws.waf":       "aws/Architecture-Service/Security-Identity-Compliance/WAF.png",
		"aws.shield":    "aws/Architecture-Service/Security-Identity-Compliance/Shield.png",
		"aws.guardduty": "aws/Architecture-Service/Security-Identity-Compliance/GuardDuty.png",
		"aws.inspector": "aws/Architecture-Service/Security-Identity-Compliance/Inspector.png",
		"aws.macie":     "aws/Architecture-Service/Security-Identity-Compliance/Macie.png",

		// AWS Developer Tools
		"aws.codecommit":   "aws/Architecture-Service/Developer-Tools/CodeCommit.png",
		"aws.codebuild":    "aws/Architecture-Service/Developer-Tools/CodeBuild.png",
		"aws.codedeploy":   "aws/Architecture-Service/Developer-Tools/CodeDeploy.png",
		"aws.codepipeline": "aws/Architecture-Service/Developer-Tools/CodePipeline.png",
		"aws.cloud9":       "aws/Architecture-Service/Developer-Tools/Cloud9.png",
		"aws.xray":         "aws/Architecture-Service/Developer-Tools/X-Ray.png",
		"aws.cloudshell":   "aws/Architecture-Service/Developer-Tools/CloudShell.png",

		// AWS Management
		"aws.cloudwatch":      "aws/Architecture-Service/Management-Governance/CloudWatch.png",
		"aws.cloudtrail":      "aws/Architecture-Service/Management-Governance/CloudTrail.png",
		"aws.config":          "aws/Architecture-Service/Management-Governance/Config.png",
		"aws.systems-manager": "aws/Architecture-Service/Management-Governance/Systems-Manager.png",
		"aws.cloudformation":  "aws/Architecture-Service/Management-Governance/CloudFormation.png",
		"aws.organizations":   "aws/Architecture-Service/Management-Governance/Organizations.png",
		"aws.control-tower":   "aws/Architecture-Service/Management-Governance/Control-Tower.png",

		// AWS IoT
		"aws.iot-core":       "aws/Architecture-Service/Internet-of-Things/IoT-Core.png",
		"aws.iot-greengrass": "aws/Architecture-Service/Internet-of-Things/IoT-Greengrass.png",
		"aws.iot-analytics":  "aws/Architecture-Service/Internet-of-Things/IoT-Analytics.png",
		"aws.iot-events":     "aws/Architecture-Service/Internet-of-Things/IoT-Events.png",
		"aws.iot-sitewise":   "aws/Architecture-Service/Internet-of-Things/IoT-SiteWise.png",

		// AWS Media Services
		"aws.mediaconnect": "aws/Architecture-Service/Media-Services/Elemental-MediaConnect.png",
		"aws.mediaconvert": "aws/Architecture-Service/Media-Services/Elemental-MediaConvert.png",
		"aws.medialive":    "aws/Architecture-Service/Media-Services/Elemental-MediaLive.png",
		"aws.mediapackage": "aws/Architecture-Service/Media-Services/Elemental-MediaPackage.png",
		"aws.mediastore":   "aws/Architecture-Service/Media-Services/Elemental-MediaStore.png",

		// AWS Migration
		"aws.dms":           "aws/Architecture-Service/Database/Database-Migration-Service.png",
		"aws.datasync":      "aws/Architecture-Service/Migration-Modernization/DataSync.png",
		"aws.transfer":      "aws/Architecture-Service/Migration-Modernization/Transfer-Family.png",
		"aws.migration-hub": "aws/Architecture-Service/Migration-Modernization/Migration-Hub.png",

		// AWS End User Computing
		"aws.workspaces": "aws/Architecture-Service/End-User-Computing/WorkSpaces-Family.png",
		"aws.appstream":  "aws/Architecture-Service/End-User-Computing/AppStream-2.png",

		// AWS Front-End Web & Mobile
		"aws.amplify":     "aws/Architecture-Service/Front-End-Web-Mobile/Amplify.png",
		"aws.device-farm": "aws/Architecture-Service/Front-End-Web-Mobile/Device-Farm.png",
		"aws.location":    "aws/Architecture-Service/Front-End-Web-Mobile/Location-Service.png",

		// AWS Architecture Groups
		"aws.region":         "aws/Architecture-Group/Region.png",
		"aws.account":        "aws/Architecture-Group/Account.png",
		"aws.cloud":          "aws/Architecture-Group/Cloud.png",
		"aws.corporate":      "aws/Architecture-Group/Corporate-data-center.png",
		"aws.subnet-public":  "aws/Architecture-Group/Public-subnet.png",
		"aws.subnet-private": "aws/Architecture-Group/Private-subnet.png",
		"aws.autoscaling":    "aws/Architecture-Group/Auto-Scaling-group.png",

		// Generic Icons (using AWS General Icons as fallback)
		"generic.server":     "aws/Resource/General-Icons/Light/Server_Light.png",
		"generic.database":   "aws/Resource/General-Icons/Light/Database_Light.png",
		"generic.client":     "aws/Resource/General-Icons/Light/Client_Light.png",
		"generic.user":       "aws/Resource/General-Icons/Light/User_Light.png",
		"generic.users":      "aws/Resource/General-Icons/Light/Users_Light.png",
		"generic.storage":    "aws/Resource/General-Icons/Light/Disk_Light.png",
		"generic.firewall":   "aws/Resource/General-Icons/Light/Firewall_Light.png",
		"generic.internet":   "aws/Resource/General-Icons/Light/Internet_Light.png",
		"generic.email":      "aws/Resource/General-Icons/Light/Email_Light.png",
		"generic.mobile":     "aws/Resource/General-Icons/Light/Mobile-client_Light.png",
		"generic.document":   "aws/Resource/General-Icons/Light/Document_Light.png",
		"generic.folder":     "aws/Resource/General-Icons/Light/Folder_Light.png",
		"generic.gear":       "aws/Resource/General-Icons/Light/Gear_Light.png",
		"generic.globe":      "aws/Resource/General-Icons/Light/Globe_Light.png",
		"generic.shield":     "aws/Resource/General-Icons/Light/Shield_Light.png",
		"generic.alert":      "aws/Resource/General-Icons/Light/Alert_Light.png",
		"generic.camera":     "aws/Resource/General-Icons/Light/Camera_Light.png",
		"generic.chat":       "aws/Resource/General-Icons/Light/Chat_Light.png",
		"generic.code":       "aws/Resource/General-Icons/Light/Source-Code_Light.png",
		"generic.question":   "aws/Resource/General-Icons/Light/Question_Light.png",
		"generic.toolkit":    "aws/Resource/General-Icons/Light/Toolkit_Light.png",
		"generic.api":        "aws/Resource/General-Icons/Light/SDK_Light.png",
		"generic.logs":       "aws/Resource/General-Icons/Light/Logs_Light.png",
		"generic.metrics":    "aws/Resource/General-Icons/Light/Metrics_Light.png",
		"generic.multimedia": "aws/Resource/General-Icons/Light/Multimedia_Light.png",
		"generic.office":     "aws/Resource/General-Icons/Light/Office-building_Light.png",
		"generic.ssl":        "aws/Resource/General-Icons/Light/SSL-padlock_Light.png",
		"generic.saml":       "aws/Resource/General-Icons/Light/SAML-token_Light.png",
		"generic.json":       "aws/Resource/General-Icons/Light/JSON-Script_Light.png",
		"generic.git":        "aws/Resource/General-Icons/Light/Git-Repository_Light.png",
		"generic.forums":     "aws/Resource/General-Icons/Light/Forums_Light.png",
		"generic.app":        "aws/Resource/General-Icons/Light/Generic-Application_Light.png",

		// Additional generic mappings
		"generic.interface": "aws/Resource/General-Icons/Light/Programming-Language_Light.png",
		"generic.security":  "aws/Resource/General-Icons/Light/Shield_Light.png",
		"generic.config":    "aws/Resource/General-Icons/Light/Gear_Light.png",
		"generic.processor": "aws/Resource/General-Icons/Light/SDK_Light.png",
		"generic.ai":        "aws/Architecture-Service/Artificial-Intelligence/Bedrock.png",
	}
}

// normalizeNodeType converts various node type formats to the standard format
func normalizeNodeType(nodeType string) string {
	// Convert to lowercase
	nodeType = strings.ToLower(nodeType)

	// Handle different separators
	nodeType = strings.ReplaceAll(nodeType, "_", ".")
	nodeType = strings.ReplaceAll(nodeType, "-", ".")
	nodeType = strings.ReplaceAll(nodeType, "::", ".")

	// Handle common aliases
	aliases := map[string]string{
		"s3":         "aws.s3",
		"ec2":        "aws.ec2",
		"rds":        "aws.rds",
		"lambda":     "aws.lambda",
		"dynamodb":   "aws.dynamodb",
		"cloudfront": "aws.cloudfront",
		"server":     "generic.server",
		"database":   "generic.database",
		"user":       "generic.user",
		"api":        "generic.api",
	}

	if mapped, exists := aliases[nodeType]; exists {
		return mapped
	}

	// Add aws prefix if it looks like an AWS service without prefix
	awsServices := []string{
		"ec2", "rds", "s3", "lambda", "dynamodb", "cloudfront",
		"sqs", "sns", "ecs", "eks", "fargate", "elasticache",
		"aurora", "redshift", "glue", "athena", "sagemaker",
	}

	for _, service := range awsServices {
		if nodeType == service {
			return "aws." + service
		}
	}

	return nodeType
}

// CleanupTempIcons removes the temporary icon directory
func CleanupTempIcons() {
	if tempIconDir != "" {
		_ = os.RemoveAll(tempIconDir)
		tempIconDir = ""
		tempIconCache = make(map[string]string)
	}
}

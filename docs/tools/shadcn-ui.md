# ShadCN UI Components Tool

The ShadCN UI tool provides comprehensive information about shadcn/ui components, including component details, usage examples, and implementation guidance.

## Overview

shadcn/ui is a popular React component library that provides beautifully designed, accessible components built with Radix UI and Tailwind CSS. This tool helps you discover, understand, and implement shadcn/ui components in your projects.

## Features

- **Component Discovery**: List and search all available components
- **Detailed Information**: Get comprehensive component details
- **Usage Examples**: Real-world implementation examples
- **Search Functionality**: Find components by name or description
- **No Dependencies**: Works without any setup or API keys

## Available Actions

| Action     | Purpose                            | Required Parameters |
|------------|------------------------------------|---------------------|
| `list`     | Get all available components       | None                |
| `search`   | Find components by keyword         | `query`             |
| `details`  | Get detailed component information | `componentName`     |
| `examples` | Get usage examples for component   | `componentName`     |

## Usage Examples

While intended to be activated via a prompt to an agent, below are some example JSON tool calls.

### List All Components
```json
{
  "name": "shadcn",
  "arguments": {
    "action": "list"
  }
}
```

### Search for Components
```json
{
  "name": "shadcn",
  "arguments": {
    "action": "search",
    "query": "button"
  }
}
```

### Get Component Details
```json
{
  "name": "shadcn",
  "arguments": {
    "action": "details",
    "componentName": "alert-dialog"
  }
}
```

### Get Usage Examples
```json
{
  "name": "shadcn",
  "arguments": {
    "action": "examples",
    "componentName": "accordion"
  }
}
```

## Component Categories

### Form Components
- **Button**: Primary and secondary actions
- **Input**: Text input fields
- **Textarea**: Multi-line text input
- **Select**: Dropdown selection
- **Checkbox**: Boolean input
- **Radio Group**: Single selection from options
- **Switch**: Toggle control
- **Slider**: Range input
- **Label**: Form field labels

### Layout Components
- **Card**: Content containers
- **Sheet**: Sliding panels
- **Dialog**: Modal dialogs
- **Popover**: Floating content
- **Tooltip**: Hover information
- **Accordion**: Collapsible content
- **Tabs**: Tabbed interfaces
- **Separator**: Visual dividers

### Navigation Components
- **Navigation Menu**: Main navigation
- **Breadcrumb**: Hierarchical navigation
- **Pagination**: Page navigation
- **Command**: Command palette
- **Context Menu**: Right-click menus
- **Dropdown Menu**: Action menus

### Feedback Components
- **Alert**: Important messages
- **Alert Dialog**: Confirmation dialogs
- **Toast**: Notification messages
- **Progress**: Loading indicators
- **Skeleton**: Loading placeholders
- **Badge**: Status indicators

### Data Display
- **Table**: Structured data
- **Avatar**: Profile images
- **Calendar**: Date selection
- **Chart**: Data visualisation
- **Carousel**: Image galleries

## Detailed Examples

### Button Component Information
```json
{
  "name": "shadcn",
  "arguments": {
    "action": "details",
    "componentName": "button"
  }
}
```

**Response includes:**
- Component description and purpose
- Available variants (default, destructive, outline, secondary, ghost, link)
- Size options (default, sm, lg, icon)
- Props and API reference
- Installation instructions
- Import statements

### Dialog Component Examples
```json
{
  "name": "shadcn",
  "arguments": {
    "action": "examples",
    "componentName": "dialog"
  }
}
```

**Response includes:**
- Basic dialog implementation
- Form dialog with validation
- Custom dialog with complex content
- Responsive dialog patterns
- Accessibility considerations

### Search for Input Components
```json
{
  "name": "shadcn",
  "arguments": {
    "action": "search",
    "query": "input"
  }
}
```

**Returns components like:**
- Input (text input)
- Textarea (multi-line input)
- Select (dropdown input)
- Checkbox (boolean input)
- Radio Group (option input)

## Common Workflows

### Component Discovery Workflow
```bash
# 1. List all available components
shadcn --action="list"

# 2. Search for specific functionality
shadcn --action="search" --query="form"

# 3. Get details for interesting components
shadcn --action="details" --componentName="form"

# 4. Get implementation examples
shadcn --action="examples" --componentName="form"
```

### Problem-Solving Workflow
```bash
# 1. Search for solution
shadcn --action="search" --query="modal"

# 2. Compare options (dialog vs sheet vs popover)
shadcn --action="details" --componentName="dialog"
shadcn --action="details" --componentName="sheet"

# 3. Get implementation guidance
shadcn --action="examples" --componentName="dialog"

# 4. Analyse implementation
think "The dialog component provides the modal functionality I need. It has built-in accessibility features and supports form integration."
```

### UI Planning Workflow
```bash
# 1. Explore available components
shadcn --action="list"

# 2. Plan component hierarchy
think "For the user dashboard, I'll need: Card for sections, Button for actions, Table for data, and Dialog for editing forms."

# 3. Get detailed implementation info
shadcn --action="examples" --componentName="card"
shadcn --action="examples" --componentName="table"

# 4. Store component decisions
memory create_entities --namespace="ui_design" --data='{"entities": [{"name": "Dashboard_Components", "observations": ["Card for sections", "Table for data display", "Dialog for forms"]}]}'
```

## Response Formats

### Component List Response
```json
{
  "components": [
    {
      "name": "accordion",
      "description": "A vertically stacked set of interactive headings",
      "category": "layout"
    },
    {
      "name": "alert",
      "description": "Displays a callout for user attention",
      "category": "feedback"
    }
  ],
  "total_components": 45
}
```

### Component Details Response
```json
{
  "name": "button",
  "description": "Displays a button or a component that looks like a button",
  "category": "form",
  "variants": [
    {"name": "default", "description": "Primary button style"},
    {"name": "destructive", "description": "For dangerous actions"},
    {"name": "outline", "description": "Secondary button style"}
  ],
  "sizes": ["default", "sm", "lg", "icon"],
  "installation": "npx shadcn-ui@latest add button",
  "import": "import { Button } from \"@/components/ui/button\"",
  "dependencies": ["@radix-ui/react-slot", "class-variance-authority"]
}
```

### Usage Examples Response
```json
{
  "component": "dialog",
  "examples": [
    {
      "title": "Basic Dialog",
      "description": "Simple dialog with trigger button",
      "code": "import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from \"@/components/ui/dialog\"\n\n<Dialog>\n  <DialogTrigger>Open</DialogTrigger>\n  <DialogContent>\n    <DialogHeader>\n      <DialogTitle>Dialog Title</DialogTitle>\n      <DialogDescription>Dialog description</DialogDescription>\n    </DialogHeader>\n  </DialogContent>\n</Dialog>"
    },
    {
      "title": "Form Dialog",
      "description": "Dialog containing a form with validation",
      "code": "// Extended form example with proper validation..."
    }
  ]
}
```

### Search Results Response
```json
{
  "query": "button",
  "results": [
    {
      "name": "button",
      "description": "Displays a button or a component that looks like a button",
      "relevance": 1.0
    },
    {
      "name": "toggle",
      "description": "A two-state button that can be either on or off",
      "relevance": 0.8
    }
  ]
}
```

## Integration Examples

### React Project Setup
```bash
# 1. Find form components needed
shadcn --action="search" --query="form"

# 2. Get implementation details
shadcn --action="examples" --componentName="form"
shadcn --action="examples" --componentName="input"

# 3. Plan component architecture
think "I need Form for validation, Input for text fields, Button for submission, and Alert for error messages."

# 4. Document component decisions
memory create_entities --namespace="project_components" --data='{"entities": [{"name": "User_Form_Components", "observations": ["Form wrapper with validation", "Input components for text", "Alert for error display"]}]}'
```

### Component Selection Workflow
```bash
# 1. Explore navigation options
shadcn --action="search" --query="navigation"

# 2. Compare different navigation components
shadcn --action="details" --componentName="navigation-menu"
shadcn --action="details" --componentName="breadcrumb"

# 3. Choose appropriate component
think "Navigation Menu is better for main navigation, while Breadcrumb is perfect for showing current page location in the hierarchy."

# 4. Get implementation examples
shadcn --action="examples" --componentName="navigation-menu"
```

### UI Consistency Planning
```bash
# 1. Review all components for consistent design
shadcn --action="list"

# 2. Group related components
think "I'll use Card for all content containers, Button variants for different action types, and consistent spacing with the provided design tokens."

# 3. Create component guidelines
memory create_entities --namespace="design_system" --data='{"entities": [{"name": "Component_Guidelines", "observations": ["Card for content containers", "Button variants for actions", "Consistent spacing patterns"]}]}'
```

## Best Practices

### Component Selection
- **Use semantic components**: Choose components that match their intended purpose
- **Consider accessibility**: All shadcn/ui components include accessibility features
- **Follow design patterns**: Use consistent patterns across your application
- **Check dependencies**: Review component dependencies before implementation

### Implementation Tips
- **Start with examples**: Use provided examples as starting points
- **Customise appropriately**: Modify components to fit your design system
- **Test thoroughly**: Ensure components work in your specific context
- **Document usage**: Keep track of component choices and customisations

### Common Use Cases
- **Forms**: Input, Textarea, Select, Button, Form wrapper
- **Navigation**: Navigation Menu, Breadcrumb, Pagination
- **Modals**: Dialog, Alert Dialog, Sheet
- **Data Display**: Table, Card, Badge, Avatar
- **Feedback**: Alert, Toast, Progress, Skeleton

---

For technical implementation details and component source code, see the [ShadCN UI source documentation](../../internal/tools/shadcnui/).

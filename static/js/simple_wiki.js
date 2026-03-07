"use strict";

$(window).load(function () {
    $("#erasePage").click(function (e) {
        e.preventDefault();
        // Use the page deletion service for a consistent confirmation flow
        window.pageDeleteService.confirmAndDeletePage(window.simple_wiki.pageName);
    });

    $("#editFrontmatter").click(function (e) {
        e.preventDefault();
        const dialog = document.querySelector('#frontmatter-dialog');
        if (dialog) {
            dialog.openDialog(window.simple_wiki.pageName);
        }
    });

    //add print menu
    addPrintMenu();

    //add inventory menu
    addInventoryMenu();

    //add page import menu
    addPageImportMenu();
});

function addPrintMenu() {
    if ($('article.content').length != 0) {
        fetch('/api/find_by_key_existence?k=label_printer')
            .then(response => response.json())
            .then(data => {
                data.ids.forEach(function (item) {
                    $("#utilityMenuSection").after(`
                    <li class="pure-menu-item">
                        <a href="#" class="pure-menu-link" onclick="printLabel('${item.identifier}')">Print ${item.title || item.identifier}</a>
                    </li>
                `);
                });
                console.log(data);
            })
            .catch(error => {
                console.error('Error:', error);
            });
    }
}

function addInventoryMenu() {
    if ($('article.content').length == 0) {
        return;
    }

    var currentPage = $('article.content').attr('id');
    if (!currentPage) {
        return;
    }

    // Check if the current page has inventory frontmatter
    fetch('/api/find_by_key_existence?k=inventory')
        .then(response => {
            if (!response.ok) {
                throw new Error('Failed to check inventory: ' + response.status);
            }
            return response.json();
        })
        .then(data => {
            // Defensive check for data structure
            if (!data || !Array.isArray(data.ids)) {
                return;
            }

            // Check if current page is in the list of pages with inventory
            var hasInventory = data.ids.some(function(item) {
                return item && item.identifier === currentPage;
            });

            if (!hasInventory) {
                return;
            }

            // Get page metadata to determine if it's a container or item
            fetch('/' + encodeURIComponent(currentPage) + '/frontmatter')
                .then(response => {
                    if (!response.ok) {
                        // Still show basic menu even if frontmatter fetch fails
                        return {};
                    }
                    return response.json();
                })
                .then(frontmatter => {
                    buildInventoryMenu(currentPage, frontmatter);
                })
                .catch(error => {
                    console.error('Error fetching frontmatter:', error);
                    // Still show basic menu on error
                    buildInventoryMenu(currentPage, {});
                });
        })
        .catch(error => {
            console.error('Error checking inventory:', error);
        });
}

function buildInventoryMenu(currentPage, frontmatter) {
    // Safely extract inventory data with defaults
    var inventory = (frontmatter && typeof frontmatter === 'object') ? frontmatter.inventory : null;
    var isContainer = inventory && (Array.isArray(inventory.items) || inventory.items !== undefined);
    var isItem = inventory && typeof inventory.container === 'string' && inventory.container !== '';
    var currentContainer = (inventory && inventory.container) || '';

    // Build sub-menu items
    var subMenuItems = [];

    // Add Item Here - only for containers
    if (isContainer) {
        subMenuItems.push(`
            <li class="pure-menu-item">
                <a href="#" class="pure-menu-link" id="inventory-add-item"><i class="fa-solid fa-plus"></i> Add Item Here</a>
            </li>
        `);
    }

    // Move This Item - only for items
    if (isItem) {
        subMenuItems.push(`
            <li class="pure-menu-item">
                <a href="#" class="pure-menu-link" id="inventory-move-item"><i class="fa-solid fa-arrows-up-down-left-right"></i> Move This Item</a>
            </li>
        `);
    }

    // Build the parent menu item with nested sub-menu
    var inventoryMenu = `
        <li class="pure-menu-item pure-menu-has-children" id="inventory-submenu">
            <a href="#" class="pure-menu-link" id="inventory-submenu-trigger"><i class="fa-solid fa-box-open"></i> Inventory</a>
            <ul class="pure-menu-children">
                ${subMenuItems.join('')}
            </ul>
        </li>
    `;

    // Insert after utility section
    $("#utilityMenuSection").after(inventoryMenu);

    // Set up click/tap toggle for sub-menu (touch device support)
    $('#inventory-submenu-trigger').on('click', function(e) {
        e.preventDefault();
        e.stopPropagation();
        $('#inventory-submenu').toggleClass('submenu-open');
    });

    // Close sub-menu when clicking outside
    $(document).on('click', function(e) {
        if (!$(e.target).closest('#inventory-submenu').length) {
            $('#inventory-submenu').removeClass('submenu-open');
        }
    });

    // Set up click handlers
    if (isContainer) {
        $('#inventory-add-item').on('click', function(e) {
            e.preventDefault();
            $('#inventory-submenu').removeClass('submenu-open');
            var dialog = document.getElementById('inventory-add-dialog');
            if (dialog && typeof dialog.openDialog === 'function') {
                dialog.openDialog(currentPage);
            }
        });
    }

    if (isItem) {
        $('#inventory-move-item').on('click', function(e) {
            e.preventDefault();
            $('#inventory-submenu').removeClass('submenu-open');
            var dialog = document.getElementById('inventory-move-dialog');
            if (dialog && typeof dialog.openDialog === 'function') {
                dialog.openDialog(currentPage, currentContainer);
            }
        });
    }
}

function addPageImportMenu() {
    if ($('article.content').length == 0) {
        return;
    }

    // Add menu item after utility menu section
    $("#utilityMenuSection").after(`
        <li class="pure-menu-item">
            <a href="#" class="pure-menu-link" id="page-import-trigger">
                <i class="fa-solid fa-file-import"></i> Import Pages
            </a>
        </li>
    `);

    $('#page-import-trigger').on('click', function(e) {
        e.preventDefault();
        var dialog = document.getElementById('page-import-dialog');
        if (dialog && typeof dialog.openDialog === 'function') {
            dialog.openDialog();
        }
    });
}

function printLabel(template_identifier) {
    var content_identifier = $('article.content').attr('id');
    fetch('/api/print_label', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({ template_identifier: template_identifier, data_identifier: content_identifier }),
    })
        .then(response => response.json())
        .then(data => {
            alert(data.message);
        })
        .catch((error) => {
            alert(error);
        });
}


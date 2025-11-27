"use strict";
var oulipo = false;

$(window).load(function () {
    // Returns a function, that, as long as it continues to be invoked, will not
    // be triggered. The function will be called after it stops being called for
    // N milliseconds. If `immediate` is passed, trigger the function on the
    // leading edge, instead of the trailing.
    function debounce(func, wait, immediate) {
        var timeout;
        return function () {
            $('#saveEditButton').removeClass()
            $('#saveEditButton').text("Editing");
            var context = this,
                args = arguments;
            var later = function () {
                timeout = null;
                if (!immediate) func.apply(context, args);
            };
            var callNow = immediate && !timeout;
            clearTimeout(timeout);
            timeout = setTimeout(later, wait);
            if (callNow) func.apply(context, args);
        };
    };

    // This will apply the debounce effect on the keyup event
    // And it only fires 500ms or half a second after the user stopped typing
    var prevText = $('#userInput').val();
    console.log("debounce: " + window.simple_wiki.debounceMS)
    $('#userInput').on('keyup', debounce(function () {
        if (prevText == $('#userInput').val()) {
            return // no changes
        }
        prevText = $('#userInput').val();

        if (oulipo) {
            $('#userInput').val($('#userInput').val().replace(/e/g, ""));
        }
        $('#saveEditButton').removeClass()
        $('#saveEditButton').text("Saving")
        upload();
    }, window.simple_wiki.debounceMS));

    var latestUpload = null, needAnother = false;
    function upload() {
        // Prevent concurrent uploads
        if (latestUpload != null) {
            needAnother = true;
            return
        }
        latestUpload = $.ajax({
            type: 'POST',
            url: '/update',
            data: JSON.stringify({
                new_text: $('#userInput').val(),
                page: window.simple_wiki.pageName,
                fetched_at: window.lastFetch,
            }),
            success: function (data) {
                latestUpload = null;

                $('#saveEditButton').removeClass()
                if (data.success == true) {
                    $('#saveEditButton').addClass("success");
                    window.lastFetch = data.unix_time;

                    if (needAnother) {
                        upload();
                    };
                } else {
                    $('#saveEditButton').addClass("failure");
                }
                $('#saveEditButton').text(data.message);
                needAnother = false;
            },
            error: function (xhr, error) {
                latestUpload = null;
                needAnother = false;
                $('#saveEditButton').removeClass()
                $('#saveEditButton').addClass("failure");
                $('#saveEditButton').text(error);
            },
            contentType: "application/json",
            dataType: 'json'
        });
    }

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

    $("textarea").keydown(function (e) {
        if (e.keyCode === 9) { // tab was pressed
            // get caret position/selection
            var start = this.selectionStart;
            var end = this.selectionEnd;

            var $this = $(this);
            var value = $this.val();

            // set textarea value to: text before caret + tab + text after caret
            $this.val(value.substring(0, start)
                + "\t"
                + value.substring(end));

            // put caret at right position again (add one for the tab)
            this.selectionStart = this.selectionEnd = start + 1;

            // prevent the focus lose
            e.preventDefault();
        }
    });

    //add print menu
    addPrintMenu();

    //add inventory menu
    addInventoryMenu();
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

    // Always add Find Item
    subMenuItems.push(`
        <li class="pure-menu-item">
            <a href="#" class="pure-menu-link" id="inventory-find-item"><i class="fa-solid fa-magnifying-glass"></i> Find Item</a>
        </li>
    `);

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
    $('#inventory-find-item').on('click', function(e) {
        e.preventDefault();
        $('#inventory-submenu').removeClass('submenu-open');
        var dialog = document.getElementById('inventory-find-dialog');
        if (dialog && typeof dialog.openDialog === 'function') {
            dialog.openDialog();
        }
    });

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

// TODO: Avoid uploading the same thing twice (check if it's already present while allowing failed uploads to be overwritten?)
function onUploadFinished(file) {
    this.removeFile(file);
    var cursorPos = $('#userInput').prop('selectionStart');
    var cursorEnd = $('#userInput').prop('selectionEnd');
    var v = $('#userInput').val();
    var textBefore = v.substring(0, cursorPos);
    var textAfter = v.substring(cursorPos, v.length);
    if (cursorEnd > cursorPos) {
        textAfter = v.substring(cursorEnd, v.length);
    }
    var prefix = '';
    if (file.type.startsWith("image")) {
        prefix = '!';
    }
    var extraText = prefix + '[' + file.xhr.getResponseHeader("Location").split('filename=')[1] + '](' +
        file.xhr.getResponseHeader("Location") +
        ')';

    $('#userInput').val(
        textBefore +
        extraText +
        textAfter
    );

    // Select the newly-inserted link
    $('#userInput').prop('selectionStart', cursorPos);
    $('#userInput').prop('selectionEnd', cursorPos + extraText.length);
    $('#userInput').trigger('keyup'); // trigger a save
}

document.addEventListener('DOMContentLoaded', () => {
    const E = ExpenseOwl;
    let config;
    let categories = [];
    let categoryTargets = {};
    let categoryParents = {};
    let recurring = [];
    const categoryList = document.getElementById('categoryList');
    const recurringForm = document.getElementById('recurringForm');
    document.getElementById('recurringStart').value = E.localDateISO();

    function renderCategories() {
        categoryList.innerHTML = categories.map((category, index) => `<div class="category-row category-target-row" data-index="${index}"><input class="category-name" value="${E.escape(category)}" aria-label="Category name"><label class="category-parent-field"><span>Parent category</span><select class="category-parent" aria-label="Parent category for ${E.escape(category)}"><option value="essentials" ${categoryParents[category] !== 'lifestyle' ? 'selected' : ''}>Essentials</option><option value="lifestyle" ${categoryParents[category] === 'lifestyle' ? 'selected' : ''}>Lifestyle</option></select></label><label class="target-input"><span>Monthly target</span><input class="category-target" type="number" min="0" step=".01" placeholder="No target" value="${categoryTargets[category] ?? ''}" aria-label="Monthly target for ${E.escape(category)}"></label><div class="category-actions"><button class="icon-button up" ${index === 0 ? 'disabled' : ''} aria-label="Move up">↑</button><button class="icon-button down" ${index === categories.length - 1 ? 'disabled' : ''} aria-label="Move down">↓</button><button class="icon-button remove" aria-label="Remove">×</button></div></div>`).join('');
        const options = categories.map(category => `<option>${E.escape(category)}</option>`).join('');
        const select = document.getElementById('recurringCategory');
        const selected = select.value;
        select.innerHTML = options;
        if (categories.includes(selected)) select.value = selected;
    }

    function syncCategories() {
        const nextCategories = [];
        const nextTargets = {};
        const nextParents = {};
        categoryList.querySelectorAll('.category-target-row').forEach(row => {
            const category = row.querySelector('.category-name').value.trim();
            const target = Number(row.querySelector('.category-target').value);
            if (!category) return;
            nextCategories.push(category);
            if (target > 0) nextTargets[category] = target;
            nextParents[category] = row.querySelector('.category-parent').value;
        });
        categories = nextCategories;
        categoryTargets = nextTargets;
        categoryParents = nextParents;
    }

    categoryList.addEventListener('click', event => {
        const row = event.target.closest('[data-index]'); if (!row) return;
        syncCategories(); const index = Number(row.dataset.index);
        if (event.target.closest('.remove')) categories.splice(index, 1);
        if (event.target.closest('.up') && index > 0) [categories[index - 1], categories[index]] = [categories[index], categories[index - 1]];
        if (event.target.closest('.down') && index < categories.length - 1) [categories[index + 1], categories[index]] = [categories[index], categories[index + 1]];
        renderCategories();
    });
    document.getElementById('addCategory').addEventListener('click', () => { syncCategories(); const input = document.getElementById('newCategory'); if (input.value.trim()) { categories.push(input.value.trim()); categoryParents[input.value.trim()] = 'essentials'; } input.value = ''; renderCategories(); });
    document.getElementById('saveHousehold').addEventListener('click', async () => { const message = document.getElementById('setupMessage'); try { syncCategories(); await E.request('/categories/edit', E.json('PUT', categories)); await Promise.all([E.request('/category-targets/edit', E.json('PUT', categoryTargets)), E.request('/category-parents/edit', E.json('PUT', categoryParents))]); config.categories = [...categories]; config.categoryTargets = { ...categoryTargets }; config.categoryParents = { ...categoryParents }; E.setMessage(message, 'Categories, groups, and targets saved.', 'success'); renderCategories(); } catch (error) { E.setMessage(message, error.message, 'error'); } });

    function renderRecurring() {
        document.getElementById('recurringList').innerHTML = recurring.length ? recurring.map(item => `<article class="recurring-row" data-id="${item.id}"><div><strong>${E.escape(item.name)}</strong><div class="lede">${E.euro(item.amount)} · ${E.escape(item.interval)} · ${item.occurrences} occurrences · ${E.ownerLabel(item.owner)}</div></div><div><button class="icon-button edit">Edit</button><button class="icon-button stop">End future</button><button class="icon-button delete">Delete all</button></div></article>`).join('') : '<div class="empty">No recurring transactions yet.</div>';
    }
    function resetRecurring() { recurringForm.reset(); document.getElementById('recurringID').value = ''; document.getElementById('recurringStart').value = E.localDateISO(); document.getElementById('recurringOccurrences').value = 12; document.getElementById('recurringSubmit').textContent = 'Add recurring transaction'; document.getElementById('cancelRecurring').hidden = true; }
    function editRecurring(item) { document.getElementById('recurringID').value = item.id; document.getElementById('recurringName').value = item.name; document.getElementById('recurringCategory').value = item.category; document.getElementById('recurringAmount').value = Math.abs(item.amount); document.getElementById('recurringStart').value = E.localDateISO(new Date(item.startDate)); document.getElementById('recurringInterval').value = item.interval; document.getElementById('recurringOccurrences').value = item.occurrences; document.getElementById('recurringOwner').value = item.owner; document.getElementById('recurringGain').checked = item.amount > 0; document.getElementById('recurringNotes').value = item.notes || ''; document.getElementById('recurringSubmit').textContent = 'Update recurring transaction'; document.getElementById('cancelRecurring').hidden = false; recurringForm.scrollIntoView({ behavior: 'smooth' }); }
    async function loadRecurring() { recurring = await E.request('/recurring-expenses'); renderRecurring(); }
    document.getElementById('cancelRecurring').addEventListener('click', resetRecurring);
    recurringForm.addEventListener('submit', async event => { event.preventDefault(); const message = document.getElementById('recurringMessage'); const id = document.getElementById('recurringID').value; let amount = Number(document.getElementById('recurringAmount').value); if (!document.getElementById('recurringGain').checked) amount *= -1; const payload = { name: document.getElementById('recurringName').value, category: document.getElementById('recurringCategory').value, amount, startDate: E.dateInputToISO(document.getElementById('recurringStart').value), interval: document.getElementById('recurringInterval').value, occurrences: Number(document.getElementById('recurringOccurrences').value), owner: document.getElementById('recurringOwner').value, notes: document.getElementById('recurringNotes').value }; try { const url = id ? `/recurring-expense/edit?id=${encodeURIComponent(id)}&updateAll=true` : '/recurring-expense'; await E.request(url, E.json('PUT', payload)); E.setMessage(message, id ? 'Recurring transaction updated.' : 'Recurring transaction added.', 'success'); resetRecurring(); await loadRecurring(); } catch (error) { E.setMessage(message, error.message, 'error'); } });
    document.getElementById('recurringList').addEventListener('click', async event => { const row = event.target.closest('[data-id]'); if (!row) return; const item = recurring.find(value => value.id === row.dataset.id); if (event.target.closest('.edit')) return editRecurring(item); const removeAll = Boolean(event.target.closest('.delete')); if (!removeAll && !event.target.closest('.stop')) return; const label = removeAll ? 'Delete this rule and every generated transaction?' : 'End this rule and remove future transactions?'; if (!confirm(label)) return; try { await E.request(`/recurring-expense/delete?id=${encodeURIComponent(item.id)}&removeAll=${removeAll}`, { method: 'DELETE' }); await loadRecurring(); } catch (error) { alert(error.message); } });

    (async () => { try { [config, recurring] = await Promise.all([E.request('/config'), E.request('/recurring-expenses')]); categories = [...config.categories]; categoryTargets = { ...(config.categoryTargets || {}) }; categoryParents = { ...(config.categoryParents || {}) }; renderCategories(); renderRecurring(); } catch (error) { E.setMessage(document.getElementById('setupMessage'), error.message, 'error'); } })();
});

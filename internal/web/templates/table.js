document.addEventListener('DOMContentLoaded', () => {
    const E = ExpenseOwl;
    let currentDate = new Date();
    let expenses = [];
    let config;
    let editing = null;
    let getOwner = E.bindOwnerRail(document.getElementById('ownerRail'), render);
    const form = document.getElementById('expenseForm');
    const editor = document.getElementById('editor');
    const filterInputs = ['search','filterCategory','filterType','filterAmount'].map(id => document.getElementById(id));
    document.getElementById('date').value = E.localDateISO();

    async function load() {
        let url = '/expenses';
        if (!document.getElementById('showAll').checked) {
            const { start, end } = E.monthBounds(currentDate);
            url += `?from=${encodeURIComponent(start.toISOString())}&to=${encodeURIComponent(end.toISOString())}`;
        }
        expenses = await E.request(url);
        render();
    }

    function filtered() {
        const owner = getOwner();
        const search = document.getElementById('search').value.trim().toLowerCase();
        const category = document.getElementById('filterCategory').value;
        const type = document.getElementById('filterType').value;
        const minimum = Number(document.getElementById('filterAmount').value || 0);
        return E.ownerExpenses(expenses, owner).filter(expense => {
            const haystack = `${expense.name} ${expense.category} ${expense.notes || ''}`.toLowerCase();
            return (!search || haystack.includes(search)) && (!category || expense.category === category) && (!type || (type === 'income' ? expense.amount > 0 : expense.amount < 0)) && Math.abs(expense.amount) >= minimum;
        });
    }

    function render() {
        const allDates = document.getElementById('showAll').checked;
        document.getElementById('currentMonth').textContent = allDates ? 'All dates' : E.monthLabel(currentDate);
        document.getElementById('prevMonth').disabled = allDates;
        document.getElementById('nextMonth').disabled = allDates;
        const items = filtered();
        document.getElementById('resultCount').textContent = `${items.length} transaction${items.length === 1 ? '' : 's'} in this view.`;
        document.getElementById('rows').innerHTML = items.map(expense => `<tr data-id="${expense.id}"><td>${E.dateTime(expense.date)}</td><td><strong>${E.escape(expense.name)}</strong>${expense.receipt ? ` <a href="${E.escape(expense.receipt)}" target="_blank" aria-label="View receipt">↗</a>` : ''}</td><td>${expense.amount > 0 ? '—' : E.escape(expense.category)}</td><td>${E.ownerLabel(expense.owner)}</td><td class="notes" title="${E.escape(expense.notes || '')}">${E.escape(expense.notes || '—')}</td><td class="amount ${expense.amount > 0 ? 'gain' : 'cost'}">${E.euro(expense.amount)}</td><td><div class="row-actions"><button class="icon-button edit" aria-label="Edit">Edit</button><button class="icon-button delete" aria-label="Delete">Delete</button></div></td></tr>`).join('');
        document.getElementById('empty').hidden = items.length !== 0;
    }

    function openEditor(expense) {
        editing = expense;
        editor.classList.add('open');
        form.reset();
        document.getElementById('name').value = expense.name;
        document.getElementById('category').value = expense.category;
        document.getElementById('amount').value = Math.abs(expense.amount);
        document.getElementById('date').value = E.localDateISO(new Date(expense.date));
        document.getElementById('gain').checked = expense.amount > 0;
        syncCategoryField();
        document.getElementById('notes').value = expense.notes || '';
        document.getElementById('removeReceiptLabel').hidden = !expense.receipt;
        document.getElementById('removeReceipt').checked = false;
        editor.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
    }
    function syncCategoryField() {
        const income = document.getElementById('gain').checked;
        const category = document.getElementById('category');
        document.getElementById('categoryField').hidden = income;
        category.required = !income;
        if (!income && !category.value && category.options.length) category.selectedIndex = 0;
    }
    function closeEditor() { editing = null; editor.classList.remove('open'); E.setMessage(document.getElementById('formMessage')); }

    document.getElementById('closeEditor').addEventListener('click', closeEditor);
    document.getElementById('cancelEdit').addEventListener('click', closeEditor);
    document.getElementById('prevMonth').addEventListener('click', async () => { currentDate = new Date(currentDate.getFullYear(), currentDate.getMonth() - 1, 15); await load(); });
    document.getElementById('nextMonth').addEventListener('click', async () => { currentDate = new Date(currentDate.getFullYear(), currentDate.getMonth() + 1, 15); await load(); });
    document.getElementById('showAll').addEventListener('change', load);
    filterInputs.forEach(input => input.addEventListener('input', render));
    document.getElementById('clearFilters').addEventListener('click', () => { filterInputs.forEach(input => input.value = ''); render(); });
    document.getElementById('gain').addEventListener('change', syncCategoryField);

    document.getElementById('rows').addEventListener('click', async event => {
        const row = event.target.closest('tr');
        if (!row) return;
        const expense = expenses.find(item => item.id === row.dataset.id);
        if (event.target.closest('.edit')) openEditor(expense);
        if (event.target.closest('.delete') && confirm(`Delete “${expense.name}”?`)) {
            try { await E.request(`/expense/delete?id=${encodeURIComponent(expense.id)}`, { method: 'DELETE' }); await load(); }
            catch (error) { alert(error.message); }
        }
    });

    form.addEventListener('submit', async event => {
        event.preventDefault();
        const message = document.getElementById('formMessage');
        const button = form.querySelector('[type=submit]');
        button.disabled = true;
        try {
            const file = document.getElementById('receipt').files[0];
            const replacement = await E.uploadReceipt(file);
            let amount = Number(document.getElementById('amount').value);
            if (!document.getElementById('gain').checked) amount *= -1;
            const receipt = replacement || (document.getElementById('removeReceipt').checked ? '' : editing.receipt || '');
            const payload = { recurringID: editing.recurringID || '', name: document.getElementById('name').value, category: amount > 0 ? '' : document.getElementById('category').value, amount, date: E.dateInputToISO(document.getElementById('date').value), owner: editing.owner || getOwner(), notes: document.getElementById('notes').value, receipt };
            const url = `/expense/edit?id=${encodeURIComponent(editing.id)}`;
            await E.request(url, E.json('PUT', payload));
            E.setMessage(message, 'Transaction saved.', 'success');
            closeEditor(); await load();
        } catch (error) { E.setMessage(message, error.message, 'error'); }
        finally { button.disabled = false; }
    });

    (async () => {
        try { config = await E.request('/config'); const options = config.categories.map(category => `<option>${E.escape(category)}</option>`).join(''); document.getElementById('category').innerHTML = options; document.getElementById('filterCategory').insertAdjacentHTML('beforeend', options); await load(); }
        catch (error) { document.getElementById('resultCount').textContent = error.message; }
    })();
});

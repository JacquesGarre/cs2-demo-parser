import { Routes } from '@angular/router';
import { UploadPageComponent } from './features/upload/upload-page.component';
import { DashboardPageComponent } from './features/dashboard/dashboard-page.component';

export const routes: Routes = [
	{ path: '', pathMatch: 'full', redirectTo: 'upload' },
	{ path: 'upload', component: UploadPageComponent },
	{ path: 'dashboard', component: DashboardPageComponent },
];
